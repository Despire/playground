package status

import (
	"bytes"
	"cmp"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"path"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer"
	"github.com/Despire/tinytorrent/p2p/peer/bitfield"
	"github.com/Despire/tinytorrent/torrent"
)

type pendingPiece struct {
	Index      uint32
	Downloaded int64
	Size       int64
	Received   []*messagesv1.Piece
	Pending    []*messagesv1.Request
	InFlight   []*messagesv1.Request
}

type requests struct {
	// l guards againts concurrent accesses
	// for the pieces map. Useful to have
	// a consistent snapshot of the pending
	// pieces when analyzed.
	l        sync.Mutex
	inFlight [4]*pendingPiece

	// free indicates if there any free slots
	// in the inFlight array.
	free atomic.Int64
}

type peers struct {
	// l guards againts concurrent accesses
	// for the peers map. Useful to have
	// a consistent snapshot.
	l       sync.Mutex
	seeders map[string]*peer.Peer
}

// Tracker wraps all necessary information for tracking
// the status of a torrent file
type Tracker struct {
	downloadDir string
	clientID    string
	logger      *slog.Logger

	requests requests
	peers    peers

	done chan struct{}
	wg   sync.WaitGroup

	Torrent    *torrent.MetaInfoFile
	BitField   *bitfield.BitField
	Uploaded   atomic.Int64
	Downloaded atomic.Int64
}

func NewTracker(clientID string, logger *slog.Logger, t *torrent.MetaInfoFile, downloadDir string) *Tracker {
	tr := Tracker{
		downloadDir: path.Join(downloadDir, hex.EncodeToString(t.Info.Metadata.Hash[:])),
		clientID:    clientID,
		logger:      logger,
		done:        make(chan struct{}),
		wg:          sync.WaitGroup{},
		Torrent:     t,
		BitField:    bitfield.NewBitfield(t.NumBlocks()),
		Uploaded:    atomic.Int64{},
		Downloaded:  atomic.Int64{},
	}

	tr.peers.seeders = make(map[string]*peer.Peer)
	tr.requests.free.Store(int64(len(tr.requests.inFlight)))

	tr.wg.Add(1)
	go tr.scheduler()
	return &tr
}

func (t *Tracker) UpdatePeers(resp *tracker.Response) error {
	t.peers.l.Lock()
	defer t.peers.l.Unlock()

	var errAll error

	for _, r := range resp.Peers {
		addr := net.JoinHostPort(r.IP, fmt.Sprint(r.Port))
		t.logger.Info("initiating connection to",
			slog.String("peer_addr", addr),
			slog.String("url", t.Torrent.Announce),
			slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
		)
		if _, ok := t.peers.seeders[addr]; ok {
			continue
		}

		blocks, overflow := t.Torrent.NumBlocks()
		np, err := peer.New(t.logger, r.PeerID, addr, blocks, overflow)
		if err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("failed to connect to peer %s: %w", addr, err))
			continue
		}

		t.wg.Add(1)
		go t.refreshPeer(np)

		t.peers.seeders[addr] = np
	}

	return errAll
}

func (t *Tracker) Close() error {
	t.peers.l.Lock()
	defer t.peers.l.Unlock()

	var errAll error

	for ip, p := range t.peers.seeders {
		if err := p.Close(); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("peer %q failed to close: %w", ip, err))
		}
	}

	close(t.done)

	t.wg.Wait()
	return errAll
}

func (t *Tracker) scheduler() {
	infoHash := string(t.Torrent.Metadata.Hash[:])
	for {
		select {
		case <-t.done:
			t.logger.Info("shutting piece downloader",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.wg.Done()
			return
		default:
			t.requests.l.Lock()
			{
				// schedule tasks.
				for _, inFlight := range t.requests.inFlight {
					if inFlight == nil {
						continue
					}

					for send := 0; send < len(inFlight.Pending); send++ {
						piece := inFlight.Pending[send]
						// select peer to contact for piece.
						var peers []*peer.Peer

						t.peers.l.Lock()
						for _, p := range t.peers.seeders {
							canRequest := p.ConnectionStatus.Load() == uint32(peer.ConnectionEstablished)
							canRequest = canRequest && p.Status.Remote.Load() == uint32(peer.UnChoked)
							canRequest = canRequest && p.Bitfield.Check(piece.Index)
							if canRequest {
								peers = append(peers, p)
							}
						}
						t.peers.l.Unlock()

						if len(peers) == 0 {
							continue
						}

						chosen := rand.IntN(len(peers))
						t.logger.Debug("sending request for piece",
							slog.String("peer", peers[chosen].Id),
							slog.String("url", t.Torrent.Announce),
							slog.String("infoHash", infoHash),
							slog.String("req", fmt.Sprintf("%#v", piece)),
						)

						if err := peers[chosen].SendRequest(piece); err != nil {
							t.logger.Error("failed to issue request",
								slog.String("peer", peers[chosen].Id),
								slog.String("url", t.Torrent.Announce),
								slog.String("infoHash", infoHash),
								slog.String("err", err.Error()),
								slog.String("req", fmt.Sprintf("%#v", piece)),
							)
							continue
						}
						// swap
						inFlight.Pending[send] = nil
						inFlight.InFlight = append(inFlight.InFlight, piece)
					}
					inFlight.Pending = slices.DeleteFunc(inFlight.Pending, func(r *messagesv1.Request) bool {
						return r == nil
					})
				}

			}
			t.requests.l.Unlock()

			freeSlots := t.requests.free.Load()
			if freeSlots == 0 {
				continue
			}
			if freeSlots < 0 {
				panic("malformed state")
			}

			// strategy: choose a random piece
			// TODO: this can be improved by choosing a rarest piece first.
			unverified := t.BitField.MissingPieces()
			nextPiece := int64(unverified[rand.IntN(len(unverified))])

			pieceStart := nextPiece * t.Torrent.PieceLength
			pieceEnd := pieceStart + t.Torrent.PieceLength
			pieceEnd = min(pieceEnd, t.Torrent.BytesToDownload())
			pieceSize := pieceEnd - pieceStart

			pending := &pendingPiece{
				Index:      uint32(nextPiece),
				Downloaded: 0,
				Size:       pieceSize,
				Received:   nil,
				Pending:    nil,
				InFlight:   nil,
			}

			for p := int64(0); p < pieceSize; {
				nextBlockSize := int64(messagesv1.RequestSize)
				if pieceSize < p+nextBlockSize {
					nextBlockSize = pieceSize - p
				}

				pending.Pending = append(pending.Pending, &messagesv1.Request{
					Index:  pending.Index,
					Begin:  uint32(p),
					Length: uint32(nextBlockSize),
				})

				p += nextBlockSize
			}

			if !t.requests.free.CompareAndSwap(freeSlots, freeSlots-1) {
				// outdated information retry again.
				continue
			}

			t.requests.l.Lock()
			slot := -1
			for i := range t.requests.inFlight {
				if t.requests.inFlight[i] == nil {
					slot = i
					break
				}
			}
			if slot < 0 {
				panic("malformed state")
			}

			t.requests.inFlight[slot] = pending
			t.requests.l.Unlock()
		}
	}
}

func (t *Tracker) recvPieces(p *peer.Peer) {
	infoHash := string(t.Torrent.Metadata.Hash[:])
	pid := p.Id
	for {
		select {
		case <-t.done:
			t.logger.Info("shutting piece downloader",
				slog.String("peer", pid),
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.wg.Done()
			return
		case recv, ok := <-p.Pieces():
			if !ok {
				t.logger.Debug("shutting piece downloader, channel closed",
					slog.String("peer", pid),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				t.wg.Done()
				return
			}

			t.requests.l.Lock()
			{
				piece := slices.IndexFunc(t.requests.inFlight[:], func(p *pendingPiece) bool {
					return p != nil && p.Index == recv.Index
				})
				if piece < 0 {
					t.logger.Debug("received piece for untracked piece index",
						slog.String("peer", pid),
						slog.String("piece_idx", fmt.Sprint(recv.Index)),
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
					)
					t.requests.l.Unlock()
					continue
				}

				pending := t.requests.inFlight[piece]

				req := slices.IndexFunc(pending.InFlight, func(r *messagesv1.Request) bool {
					req := messagesv1.Request{
						Index:  recv.Index,
						Begin:  recv.Begin,
						Length: uint32(len(recv.Block)),
					}
					return r != nil && *r == req
				})
				if req < 0 {
					t.logger.Debug("received piece for untracked piece",
						slog.String("peer", pid),
						slog.String("piece_idx", fmt.Sprint(recv.Index)),
						slog.String("piece_offset", fmt.Sprint(recv.Begin)),
						slog.String("piece_length", fmt.Sprint(len(recv.Block))),
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
					)
					t.requests.l.Unlock()
					continue
				}

				// check for duplicates
				for _, other := range pending.Received {
					duplicate := other.Begin == recv.Begin
					duplicate = duplicate && other.Index == recv.Index
					duplicate = duplicate && len(other.Block) == len(recv.Block)
					if duplicate {
						panic(fmt.Sprintf("malformed state recieved duplicate piece: %v %v %v", recv.Index, recv.Begin, len(recv.Block)))
					}
				}

				pending.Downloaded += int64(len(recv.Block))
				if pending.Downloaded > pending.Size {
					panic(fmt.Sprintf("recieved more data than expected for piece %v", recv.Index))
				}

				// move to pending
				pending.Received = append(pending.Received, recv)
				pending.InFlight = slices.Delete(pending.InFlight, req, req+1)

				status := float64(pending.Downloaded) / float64(pending.Size)
				status *= 100
				t.logger.Debug("received piece",
					slog.String("peer", pid),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
					slog.String("piece", fmt.Sprint(recv.Index)),
					slog.String("downloaded_bytes", fmt.Sprint(pending.Downloaded)),
					slog.String("status", fmt.Sprintf("%.2f%%", status)),
				)

				if pending.Downloaded == pending.Size { // verify
					slices.SortFunc(pending.Received, func(a, b *messagesv1.Piece) int { return cmp.Compare(a.Begin, b.Begin) })
					var data []byte
					for _, d := range pending.Received {
						data = append(data, d.Block...)
					}
					digest := sha1.Sum(data)

					if !bytes.Equal(digest[:], t.Torrent.PieceHash(recv.Index)) {
						t.logger.Debug("invalid piece sha1 hash, stop tracking",
							slog.String("peer", pid),
							slog.String("piece", fmt.Sprint(recv.Index)),
							slog.String("url", t.Torrent.Announce),
							slog.String("infoHash", infoHash),
						)
						// TODO: maybe close connection ?

						t.requests.inFlight[piece] = nil // garbage collect.
						t.requests.free.Add(+1)
						t.requests.l.Unlock()
						continue
					}

					if err := t.Flush(recv.Index, data); err != nil {
						t.logger.Debug("failed to flush piece",
							slog.String("peer", pid),
							slog.String("url", t.Torrent.Announce),
							slog.String("infoHash", infoHash),
							slog.String("err", err.Error()),
							slog.String("piece", fmt.Sprint(recv.Index)),
						)
						t.requests.l.Unlock()
						continue
					}

					t.Downloaded.Add(pending.Size)
					t.logger.Info("piece verified successfully",
						slog.String("peer", pid),
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
						slog.String("piece", fmt.Sprint(recv.Index)),
						slog.String("downloaded_bytes", fmt.Sprint(t.Downloaded.Load())),
						slog.String("status", fmt.Sprintf("%.2f%%", (float64(t.Downloaded.Load())/float64(t.Torrent.BytesToDownload()))*100)),
					)

					t.requests.inFlight[piece] = nil
					t.requests.free.Add(+1)
					t.BitField.Set(recv.Index)
				}
			}
			t.requests.l.Unlock()
		}
	}
}

func (t *Tracker) Flush(idx uint32, pieceBytes []byte) error {
	if _, err := os.Stat(t.downloadDir); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(t.downloadDir, os.ModePerm); err != nil {
			return err
		}
	}

	piece := path.Join(t.downloadDir, fmt.Sprint(idx))

	f, err := os.Create(piece)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := io.Copy(f, bytes.NewReader(pieceBytes))
	if err != nil {
		return err
	}

	if int(w) != len(pieceBytes) {
		return fmt.Errorf("failed to write all piece bytes to disk %d out of %d", w, len(pieceBytes))
	}

	return nil
}

func (t *Tracker) refreshPeer(p *peer.Peer) {
	refresh := time.NewTicker(1 * time.Nanosecond) // first tick happens immediately.
	infoHash := string(t.Torrent.Metadata.Hash[:])
	for {
		select {
		case <-t.done:
			t.logger.Info("shutting down peer refresher",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
				slog.String("peer", p.Id),
			)
			t.wg.Done()
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch s := peer.ConnectionStatus(p.ConnectionStatus.Load()); s {
			case peer.ConnectionPending:
				t.logger.Info("initiating handshake",
					slog.String("peer_ip", p.Addr),
					slog.String("peer_addr", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
				)
				if err := p.InitiateHandshakeV1(string(t.Torrent.Metadata.Hash[:]), t.clientID); err != nil {
					t.logger.Error("failed to initiating handshake, skipping to next peer",
						slog.String("peer_ip", p.Addr),
						slog.String("peer_addr", p.Id),
						slog.String("url", t.Torrent.Announce),
						slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
						slog.String("err", err.Error()),
					)
					if err := p.Close(); err != nil {
						t.logger.Error("failed to close peer",
							slog.String("peer_ip", p.Addr),
							slog.String("peer_addr", p.Id),
							slog.String("url", t.Torrent.Announce),
							slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
							slog.String("err", err.Error()),
						)
					}
					continue
				}
				if err := p.SendBitfield(t.BitField.Clone()); err != nil {
					t.logger.Error("failed to send bitfield msg",
						slog.String("peer_ip", p.Addr),
						slog.String("peer_addr", p.Id),
						slog.String("url", t.Torrent.Announce),
						slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
					)
				}
				if err := p.SendInterested(); err != nil {
					t.logger.Error("failed to send interested msg",
						slog.String("peer_ip", p.Addr),
						slog.String("peer_addr", p.Id),
						slog.String("url", t.Torrent.Announce),
						slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
					)
				}
				// Listen for pieces.
				t.wg.Add(1)
				go t.recvPieces(p)
			case peer.ConnectionKilled:
				t.logger.Info("attempting to re-connect with peer",
					slog.String("peer_ip", p.Addr),
					slog.String("peer_addr", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
				)
				if err := p.Reconnect(); err != nil {
					t.logger.Error("failed to reconnect with peer",
						slog.String("peer_ip", p.Addr),
						slog.String("peer_addr", p.Id),
						slog.String("url", t.Torrent.Announce),
						slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
					)
					continue
				}
			case peer.ConnectionEstablished:
				t.logger.Info("sending keep alive event on torrent peers",
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				if err := p.SendKeepAlive(); err != nil {
					t.logger.Error("failed to keep alive",
						slog.String("err", err.Error()),
						slog.String("infoHash", infoHash),
						slog.String("url", t.Torrent.Announce),
						slog.String("peer", p.Id),
					)
				}
			}
		}
	}
}
