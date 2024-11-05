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

func (t *Tracker) CancelDownload()                      { close(t.download.cancel); t.download.wg.Wait() }
func (t *Tracker) WaitUntilDownloaded() <-chan struct{} { return t.download.completed }

func (t *Tracker) UpdateSeeders(resp *tracker.Response) error {
	if t.Downloaded.Load() == t.Torrent.BytesToDownload() {
		return nil
	}

	var errAll error

	for _, r := range resp.Peers {
		addr := net.JoinHostPort(r.IP, fmt.Sprint(r.Port))
		t.logger.Debug("initiating connection to peer", slog.String("addr", addr))
		if _, ok := t.peers.seeders.Load(addr); ok {
			continue
		}

		t.download.wg.Add(1)
		go t.keepAliveSeeders(addr)
	}

	return errAll
}

func (t *Tracker) downloadScheduler() {
	defer t.download.wg.Done()

	unverified := make(map[uint32]struct{})
	for _, i := range t.BitField.MissingPieces() {
		unverified[i] = struct{}{}
	}

	currentRate := int64(0)
	rateTicker := time.NewTicker(rateTick)
	for {
		select {
		case <-t.stop:
			t.logger.Info("shutting down piece downloader, closed tracker")
			return
		case <-t.download.cancel:
			t.logger.Info("shutting down piece downloader, canceled download")
			return
		case <-rateTicker.C:
			newRate := t.Downloaded.Load()
			diff := max(0, newRate-currentRate)
			t.download.rate.Store(diff)
			currentRate = newRate
		default:
			freeSlots := 0
			for i := range t.download.requests {
				p := t.download.requests[i].Load()
				if p == nil {
					freeSlots++
					continue
				}

				p.l.Lock()

				// reschedule long running requests.
				for send := 0; send < len(p.InFlight); send++ {
					if req := p.InFlight[send]; !req.received && time.Since(req.send) > 8*time.Second {
						t.peers.seeders.Range(func(_, value any) bool {
							p := value.(*peer.Peer)
							canCancel := p.ConnectionStatus() == peer.ConnectionEstablished
							canCancel = canCancel && p.Status.Remote.Load() == uint32(peer.UnChoked)
							if canCancel {
								err := p.SendCancel(&messagesv1.Cancel{
									Index:  req.request.Index,
									Begin:  req.request.Begin,
									Length: req.request.Length,
								})
								if err != nil {
									t.logger.Error("failed to cancel request",
										slog.Any("err", err),
										slog.String("end_peer", p.Id),
										slog.String("req", fmt.Sprintf("%#v", req)),
									)
								}
							}
							return true
						})

						p.Pending = append(p.Pending, &messagesv1.Request{
							Index:  req.request.Index,
							Begin:  req.request.Begin,
							Length: req.request.Length,
						})
						p.InFlight[send] = nil
					}
				}
				p.InFlight = slices.DeleteFunc(p.InFlight, func(r *timedDownloadRequest) bool { return r == nil })

				// schedule pending requests to peers.
				for send := 0; send < len(p.Pending); send++ {
					piece := p.Pending[send]
					// select peer to contact for piece.
					var peers []*peer.Peer

					t.peers.seeders.Range(func(_, value any) bool {
						p := value.(*peer.Peer)
						canRequest := p.ConnectionStatus() == peer.ConnectionEstablished
						canRequest = canRequest && p.Status.Remote.Load() == uint32(peer.UnChoked)
						canRequest = canRequest && p.Bitfield.Check(piece.Index)
						if canRequest {
							peers = append(peers, p)
						}
						return true
					})

					if len(peers) == 0 {
						t.logger.Debug("no peers online that contain needed piece",
							slog.String("piece", fmt.Sprint(piece.Index)),
							slog.String("req", fmt.Sprintf("%#v", piece)),
						)
						continue
					}

					chosen := rand.IntN(len(peers))
					t.logger.Debug("sending request for piece",
						slog.String("end_peer", peers[chosen].Id),
						slog.String("req", fmt.Sprintf("%#v", piece)),
					)

					if err := peers[chosen].SendRequest(piece); err != nil {
						t.logger.Error("failed to issue request",
							slog.Any("err", err),
							slog.String("end_peer", peers[chosen].Id),
							slog.String("req", fmt.Sprintf("%#v", piece)),
						)
						continue
					}

					p.Pending[send] = nil
					p.InFlight = append(p.InFlight, &timedDownloadRequest{
						request: *piece,
						send:    time.Now(),
					})
				}
				p.Pending = slices.DeleteFunc(p.Pending, func(r *messagesv1.Request) bool { return r == nil })
				p.l.Unlock()
			}

			if len(unverified) == 0 { // we can't process any new pieces, wait for pending to finish.
				if freeSlots == len(t.download.requests) {
					t.logger.Info("Downloaded all pieces shutting down piece downloader")
					close(t.download.completed)
					return
				}
				time.Sleep(250 * time.Millisecond)
				continue
			}

			slot := -1
			for i := range t.download.requests {
				if t.download.requests[i].Load() == nil {
					slot = i
					break
				}
			}
			if slot < 0 {
				// no free slot
				time.Sleep(250 * time.Millisecond)
				continue
			}

			index := int64(-1)
			// find the next missing piece that can be downloaded
			for unverified := range unverified {
				t.peers.seeders.Range(func(_, value any) bool {
					p := value.(*peer.Peer)
					if p.Bitfield.Check(unverified) {
						index = int64(unverified)
						return false
					}
					return true
				})
			}

			if index < 0 {
				// no peers available for any piece to download
				time.Sleep(5 * time.Second)
				continue
			}

			pieceStart := index * t.Torrent.PieceLength
			pieceEnd := pieceStart + t.Torrent.PieceLength
			pieceEnd = min(pieceEnd, t.Torrent.BytesToDownload())
			pieceSize := pieceEnd - pieceStart

			pending := &pendingPiece{
				Index:      uint32(index),
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

			if !t.download.requests[slot].CompareAndSwap(nil, pending) {
				continue // slot was taken away.
			}

			delete(unverified, uint32(index))
		}
	}
}

func (t *Tracker) recvPieces(logger *slog.Logger, pieces <-chan *messagesv1.Piece) {
	defer t.download.wg.Done()
	for {
		select {
		case recv, ok := <-pieces:
			if !ok {
				logger.Debug("shutting piece downloader, channel closed")
				return
			}

			pieceIdx := -1
			var piece *pendingPiece
			for i := range t.download.requests {
				if r := t.download.requests[i].Load(); r != nil && r.Index == recv.Index {
					piece = r
					pieceIdx = i
					break
				}
			}
			if piece == nil {
				logger.Debug("received piece for untracked piece index", slog.String("piece_idx", fmt.Sprint(recv.Index)))
				continue
			}

			piece.l.Lock()

			req := slices.IndexFunc(piece.InFlight, func(r *timedDownloadRequest) bool {
				return r.request == messagesv1.Request{
					Index:  recv.Index,
					Begin:  recv.Begin,
					Length: uint32(len(recv.Block)),
				}
			})

			if req < 0 {
				logger.Debug("received piece for untracked piece",
					slog.String("piece_idx", fmt.Sprint(recv.Index)),
					slog.String("piece_offset", fmt.Sprint(recv.Begin)),
					slog.String("piece_length", fmt.Sprint(len(recv.Block))),
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
			total := t.Downloaded.Add(int64(len(recv.Block)))

			piece.Received = append(piece.Received, recv)
			piece.InFlight[req].received = true // mark as received to it won't be rescheduled again.

			status := float64(piece.Downloaded) / float64(piece.Size)
			status *= 100
			logger.Debug("received piece",
				slog.String("piece", fmt.Sprint(recv.Index)),
				slog.String("downloaded_bytes", fmt.Sprint(piece.Downloaded)),
				slog.String("status", fmt.Sprintf("%.2f%%", status)),
			)

			if piece.Downloaded == piece.Size {
				slices.SortFunc(piece.Received, func(a, b *messagesv1.Piece) int { return cmp.Compare(a.Begin, b.Begin) })
				var data []byte
				for _, d := range piece.Received {
					data = append(data, d.Block...)
				}
				digest := sha1.Sum(data)

				if !bytes.Equal(digest[:], t.Torrent.PieceHash(recv.Index)) {
					logger.Error("invalid piece sha1 hash, retrying", slog.String("piece", fmt.Sprint(recv.Index)))
					// TODO: mark peer as malicious and close connection.
					t.Downloaded.Add(-piece.Size)
					if err := piece.Retry(); err != nil {
						piece.l.Unlock()
						panic("malformed state, expected no pending requests when rescheduling piece for retry download")
					}
					piece.l.Unlock()
					continue
				}

				if err := t.Flush(recv.Index, data); err != nil {
					logger.Error("failed to flush piece", slog.Any("err", err), slog.String("piece", fmt.Sprint(recv.Index)))
					t.Downloaded.Add(-piece.Size)
					if err := piece.Retry(); err != nil {
						piece.l.Unlock()
						panic("malformed state, expected no pending requests when rescheduling piece for retry download")
					}
					piece.l.Unlock()
					continue
				}

				t.BitField.Set(recv.Index)

				logger.Debug("sending have message for verified piece", slog.String("piece", fmt.Sprint(recv.Index)))

				// send have message to all peers.
				t.peers.seeders.Range(func(_, value any) bool {
					if p := value.(*peer.Peer); p.ConnectionStatus() == peer.ConnectionEstablished {
						if err := p.SendHave(&messagesv1.Have{Index: recv.Index}); err != nil {
							logger.Error("failed to send have piece, after verifying", slog.Any("err", err),
								slog.String("end_peer", p.Id),
								slog.String("piece", fmt.Sprint(recv.Index)),
							)
						}
					}
					return true
				})
				t.peers.leechers.Range(func(_, value any) bool {
					if p := value.(*peer.Peer); p.ConnectionStatus() == peer.ConnectionEstablished {
						if err := p.SendHave(&messagesv1.Have{Index: recv.Index}); err != nil {
							logger.Error("failed to send have piece, after verifying", slog.Any("err", err),
								slog.String("end_peer", p.Id),
								slog.String("piece", fmt.Sprint(recv.Index)),
							)
						}
					}
					return true
				})

				logger.Info("piece verified successfully",
					slog.String("status", fmt.Sprintf("%.2f%%", (float64(total)/float64(t.Torrent.BytesToDownload()))*100)),
					slog.String("kbps", fmt.Sprintf("%.2f", (float64(t.download.rate.Load())/1000.0)*100)),
					slog.String("piece", fmt.Sprint(recv.Index)),
				)

				// make place for a new piece to be scheduled.
				if !t.download.requests[pieceIdx].CompareAndSwap(piece, nil) {
					logger.Warn("two go-routines verified same piece", slog.String("piece", fmt.Sprint(recv.Index)))
				}
			}

			piece.l.Unlock()
		}
	}
}

func (t *Tracker) keepAliveSeeders(addr string) {
	logger := t.logger.With(slog.String("peer_ip", addr))

	var p *peer.Peer
	defer func() {
		if err := p.SendNotInterested(); err != nil {
			logger.Error("failed to send not-interested msg", slog.Any("err", err))
		}
		if err := p.Close(); err != nil {
			logger.Error("failed to close peer", slog.Any("err", err))
		}

		t.download.wg.Done()
	}()

	refresh := time.NewTicker(1 * time.Nanosecond) // first tick happens immediately.
	for {
		select {
		case <-t.stop:
			logger.Debug("shutting down peer refresher, stopped tracker")
			return
		case <-t.download.cancel:
			logger.Debug("shutting down peer refresher, canceled download")
			return
		case <-t.download.completed:
			logger.Debug("shutting down peer refresher, as torrent was downloaded")
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch p.ConnectionStatus() {
			case peer.ConnectionKilled:
				if err := p.Close(); err != nil {
					logger.Error("failed to close peer", slog.Any("err", err))
				}
				t.peers.seeders.Delete(addr)

				var err error
				p, err = peer.NewSeederConnection(
					logger,
					addr,
					t.Torrent.NumPieces(),
					string(t.Torrent.Metadata.Hash[:]),
					t.clientID,
				)
				if err != nil {
					logger.Error("failed to initiating handshake", slog.Any("err", err))
					continue
				}

				t.peers.seeders.Store(addr, p)

				// Listen for incoming pieces.
				t.download.wg.Add(1)
				go t.recvPieces(logger.With(slog.String("pid", p.Id)), p.Pieces())

				if err := p.SendBitfield(t.BitField.Clone()); err != nil {
					logger.Error("failed to send bitfield msg")
				}

				if err := p.SendInterested(); err != nil {
					logger.Error("failed to send interested msg")
				}
			case peer.ConnectionEstablished:
				logger.Debug("sending keep alive event on torrent peer")
				if err := p.SendKeepAlive(); err != nil {
					logger.Error("failed to keep alive, closing", slog.Any("err", err))
					if err := p.Close(); err != nil {
						logger.Error("failed to close peer", slog.Any("err", err))
					}
				}
			}
		}
	}
}
