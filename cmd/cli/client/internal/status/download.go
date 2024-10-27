package status

import (
	"bytes"
	"cmp"
	"crypto/sha1"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"slices"
	"time"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer"
)

func (t *Tracker) CancelDownload()                      { close(t.cancelDownload); t.downloadWg.Wait() }
func (t *Tracker) WaitUntilDownloaded() <-chan struct{} { return t.downloaded }

func (t *Tracker) UpdateSeeders(resp *tracker.Response) error {
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
		t.peers.seeders[addr] = peer.New(t.logger, r.PeerID, addr, blocks, overflow)
		t.downloadWg.Add(1)
		go t.keepAliveSeeders(t.peers.seeders[addr])
	}

	return errAll
}

func (t *Tracker) downloadScheduler() {
	infoHash := string(t.Torrent.Metadata.Hash[:])
	// pool in random order
	// TODO: change to rarest piece.
	unverified := t.BitField.MissingPieces()
	rand.Shuffle(len(unverified), func(i, j int) {
		unverified[i], unverified[j] = unverified[j], unverified[i]
	})

	for {
		select {
		case <-t.stop:
			t.logger.Info("shutting down piece downloader, closed tracker",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.downloadWg.Done()
			return
		case <-t.cancelDownload:
			t.logger.Info("shutting down piece downloader, canceled download",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.downloadWg.Done()
			return
		default:
			freeSlots := 0
			for i := range t.requests {
				p := t.requests[i].Load()
				if p == nil {
					freeSlots++
					continue
				}

				p.l.Lock()
				for send := 0; send < len(p.Pending); send++ {
					piece := p.Pending[send]
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
						t.logger.Warn("no peers online that contain needed piece",
							slog.String("piece", fmt.Sprint(piece.Index)),
							slog.String("url", t.Torrent.Announce),
							slog.String("infoHash", infoHash),
							slog.String("req", fmt.Sprintf("%#v", piece)),
						)
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

					p.Pending[send] = nil
					p.InFlight = append(p.InFlight, piece)
				}
				p.Pending = slices.DeleteFunc(p.Pending, func(r *messagesv1.Request) bool { return r == nil })
				p.l.Unlock()
			}

			if len(unverified) == 0 { // we can't process any new pieces, wait for pending to finish.
				if freeSlots == len(t.requests) {
					t.logger.Info("Downloaded all pieces shutting down piece downloader",
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
					)
					close(t.downloaded)
					t.downloadWg.Done()
					return
				}
				continue
			}

			slot := -1
			for i := range t.requests {
				if t.requests[i].Load() == nil {
					slot = i
					break
				}
			}
			if slot < 0 {
				// no free slot
				continue
			}

			pieceStart := int64(unverified[0]) * t.Torrent.PieceLength
			pieceEnd := pieceStart + t.Torrent.PieceLength
			pieceEnd = min(pieceEnd, t.Torrent.BytesToDownload())
			pieceSize := pieceEnd - pieceStart

			pending := &pendingPiece{
				Index:      unverified[0],
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

			if !t.requests[slot].CompareAndSwap(nil, pending) {
				continue // slot was taken away.
			}

			unverified = unverified[1:]
		}
	}
}

func (t *Tracker) recvPieces(p *peer.Peer) {
	infoHash := string(t.Torrent.Metadata.Hash[:])
	pid := p.Id
	for {
		select {
		case recv, ok := <-p.Pieces():
			if !ok {
				t.logger.Debug("shutting piece downloader, channel closed",
					slog.String("peer", pid),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				t.downloadWg.Done()
				return
			}

			pieceIdx := -1
			var piece *pendingPiece
			for i := range t.requests {
				if r := t.requests[i].Load(); r != nil && r.Index == recv.Index {
					piece = r
					pieceIdx = i
					break
				}
			}
			if piece == nil {
				t.logger.Debug("received piece for untracked piece index",
					slog.String("peer", pid),
					slog.String("piece_idx", fmt.Sprint(recv.Index)),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				continue
			}

			piece.l.Lock()

			req := slices.IndexFunc(piece.InFlight, func(r *messagesv1.Request) bool {
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
				piece.l.Unlock()
				continue
			}

			// check for duplicates
			var skip bool
			for _, other := range piece.Received {
				duplicate := other.Begin == recv.Begin
				duplicate = duplicate && other.Index == recv.Index
				duplicate = duplicate && len(other.Block) == len(recv.Block)
				if duplicate {
					skip = true
					break
				}
			}
			if skip {
				piece.l.Unlock()
				continue
			}

			piece.Downloaded += int64(len(recv.Block))
			if piece.Downloaded > piece.Size {
				piece.l.Unlock()
				panic(fmt.Sprintf("recieved more data than expected for piece %v", recv.Index))
			}

			piece.Received = append(piece.Received, recv)

			status := float64(piece.Downloaded) / float64(piece.Size)
			status *= 100
			t.logger.Debug("received piece",
				slog.String("peer", pid),
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
				slog.String("piece", fmt.Sprint(recv.Index)),
				slog.String("downloaded_bytes", fmt.Sprint(piece.Downloaded)),
				slog.String("status", fmt.Sprintf("%.2f%%", status)),
			)

			if piece.Downloaded == piece.Size { // verify
				slices.SortFunc(piece.Received, func(a, b *messagesv1.Piece) int { return cmp.Compare(a.Begin, b.Begin) })
				var data []byte
				for _, d := range piece.Received {
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
					if err := p.Close(); err != nil {
						t.logger.Error("failed to close peer after invalid sha1 hash",
							slog.String("peer", pid),
							slog.String("piece", fmt.Sprint(recv.Index)),
							slog.String("url", t.Torrent.Announce),
							slog.String("infoHash", infoHash),
						)
					}
					// retry downloading the piece again.
					if len(piece.Pending) != 0 {
						piece.l.Unlock()
						panic("malformed state, expected no pending requests when rescheduling piece for retry download")
					}
					piece.Pending = append(piece.Pending, piece.InFlight...)
					piece.Received = nil
					piece.l.Unlock()
					continue
				}

				if err := t.Flush(recv.Index, data); err != nil {
					t.logger.Error("failed to flush piece",
						slog.String("peer", pid),
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
						slog.String("err", err.Error()),
						slog.String("piece", fmt.Sprint(recv.Index)),
					)
					// retry downloading the piece again.
					if len(piece.Pending) != 0 {
						piece.l.Unlock()
						panic("malformed state, expected no pending requests when rescheduling piece for retry download")
					}
					piece.Pending = append(piece.Pending, piece.InFlight...)
					piece.Received = nil
					piece.l.Unlock()
					continue
				}

				t.BitField.Set(recv.Index)

				t.peers.l.Lock()
				t.logger.Debug("sending have message for verified piece",
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
					slog.String("piece", fmt.Sprint(recv.Index)),
				)
				for _, p := range t.peers.seeders {
					if p.ConnectionStatus.Load() == uint32(peer.ConnectionEstablished) {
						if err := p.SendHave(&messagesv1.Have{Index: recv.Index}); err != nil {
							t.logger.Error("failed to send have piece, after verifying",
								slog.String("end_peer", p.Id),
								slog.String("url", t.Torrent.Announce),
								slog.String("infoHash", infoHash),
								slog.String("err", err.Error()),
								slog.String("piece", fmt.Sprint(recv.Index)),
							)
						}
					}
				}
				t.peers.l.Unlock()

				total := t.Downloaded.Add(piece.Size)
				t.logger.Info("piece verified successfully",
					slog.String("status", fmt.Sprintf("%.2f%%", (float64(total)/float64(t.Torrent.BytesToDownload()))*100)),
					slog.String("piece", fmt.Sprint(recv.Index)),
					slog.String("peer", pid),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)

				if !t.requests[pieceIdx].CompareAndSwap(piece, nil) {
					t.logger.Warn("two go-routines verified same piece",
						slog.String("peer", pid),
						slog.String("url", t.Torrent.Announce),
						slog.String("infoHash", infoHash),
						slog.String("piece", fmt.Sprint(recv.Index)),
					)
				}
			}

			piece.l.Unlock()
		}
	}
}

func (t *Tracker) keepAliveSeeders(p *peer.Peer) {
	refresh := time.NewTicker(1 * time.Nanosecond) // first tick happens immediately.
	infoHash := string(t.Torrent.Metadata.Hash[:])
	for {
		select {
		case <-t.stop:
			t.logger.Info("shutting down peer refresher, stopped tracker",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
				slog.String("peer", p.Id),
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
			t.downloadWg.Done()
			return
		case <-t.cancelDownload:
			t.logger.Info("shutting down peer refresher, canceled download",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
				slog.String("peer", p.Id),
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
			t.downloadWg.Done()
			return
		case <-t.downloaded:
			t.logger.Debug("shutting down peer refresher, as torrent was downloaded",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
				slog.String("peer", p.Id),
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
			t.downloadWg.Done()
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch s := peer.ConnectionStatus(p.ConnectionStatus.Load()); s {
			case peer.ConnectionPending, peer.ConnectionKilled:
				t.logger.Info("attempting to connect with peer",
					slog.String("peer_ip", p.Addr),
					slog.String("peer_addr", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
				)
				if err := p.Connect(); err != nil {
					t.logger.Error("failed to reconnect with peer",
						slog.String("peer_ip", p.Addr),
						slog.String("peer_addr", p.Id),
						slog.String("url", t.Torrent.Announce),
						slog.String("err", err.Error()),
						slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
					)
					continue
				}
				t.logger.Info("initiating handshake",
					slog.String("peer_ip", p.Addr),
					slog.String("peer_addr", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
				)
				if err := p.InitiateHandshakeV1(string(t.Torrent.Metadata.Hash[:]), t.clientID); err != nil {
					t.logger.Error("failed to initiating handshake",
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
				t.downloadWg.Add(1)
				go t.recvPieces(p)
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
