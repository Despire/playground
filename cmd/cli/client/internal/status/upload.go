package status

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer"
)

func (t *Tracker) CancelUpload() { close(t.upload.cancel); t.upload.wg.Wait() }

func (t *Tracker) AddLeecher(id string, conn net.Conn) error {
	np := peer.NewLeecher(
		t.logger,
		id,
		conn.RemoteAddr().String(),
		t.Torrent.NumPieces(),
		conn,
	)

	infoHash := string(t.Torrent.Metadata.Hash[:])
	if err := np.SendHandshakeV1(infoHash, t.clientID); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}
	if err := np.SendBitfield(t.BitField.Clone()); err != nil {
		return fmt.Errorf("failed to send bitfield")
	}

	t.peers.leechers.Store(conn.RemoteAddr().String(), np)
	t.upload.wg.Add(2)
	go t.keepAliveLeechers(np)
	go t.handleRequests(np)
	return nil
}

func (t *Tracker) processUploadRequests() {
	infoHash := string(t.Torrent.Metadata.Hash[:])

	currentRate := int64(0)
	rateTicker := time.NewTicker(rateTick)
	for {
		select {
		case <-t.stop:
			t.logger.Info("shutting down piece uploader, closed tracker",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.upload.wg.Done()
			return
		case <-t.upload.cancel:
			t.logger.Info("shutting down piece uploader, canceled upload",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			t.upload.wg.Done()
			return
		case <-rateTicker.C:
			newRate := t.Uploaded.Load()
			diff := newRate - currentRate
			t.upload.rate.Store(diff)
			currentRate = newRate
		default:
			for i := range t.upload.requests {
				req := t.upload.requests[i].Load()
				if req == nil {
					continue
				}
				t.peers.leechers.Range(func(key, value any) bool {
					if key.(string) == req.addr {
						p := value.(*peer.Peer)

						b, err := t.ReadRequest(&req.request)
						if err != nil {
							t.upload.requests[i].CompareAndSwap(req, nil)
							return false
						}
						err = p.SendPiece(&messagesv1.Piece{
							Index: req.request.Index,
							Begin: req.request.Begin,
							Block: b,
						})
						if err != nil {
							t.upload.requests[i].CompareAndSwap(req, nil)
							return false
						}
						t.upload.requests[i].CompareAndSwap(req, nil)
						return false
					}
					return true
				})
			}
		}
	}
}

func (t *Tracker) handleRequests(p *peer.Peer) {
	infoHash := string(t.Torrent.Metadata.Hash[:])
	requests, cancels := p.LeecherRequests()
	for {
		select {
		case c, ok := <-cancels:
			if !ok {
				t.logger.Debug("shutting request handler, channel closed",
					slog.String("peer", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				t.upload.wg.Done()
				return
			}
			for i := range t.upload.requests {
				req := t.upload.requests[i].Load()
				if req == nil {
					continue
				}
				matched := req.request.Index == c.Index &&
					req.request.Begin == c.Begin &&
					req.request.Length == c.Length
				if matched {
					t.upload.requests[i].CompareAndSwap(req, nil)
				}
			}
		case r, ok := <-requests:
			if !ok {
				t.logger.Debug("shutting request handler, channel closed",
					slog.String("peer", p.Id),
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
				)
				t.upload.wg.Done()
				return
			}

			if !t.BitField.Check(r.Index) {
				continue // we don't have the piece.
			}

			timedUpload := &timedUploadRequest{
				request: messagesv1.Request{
					Index:  r.Index,
					Begin:  r.Begin,
					Length: r.Length,
				},
				recieved: time.Now(),
				addr:     p.Addr,
			}

			for { // attempt to store the request while there is some free slot.
				slot := -1
				for i := range t.upload.requests {
					if t.upload.requests[i].Load() == nil {
						slot = i
						break
					}
				}
				if slot < 0 {
					// no free slot
					break
				}
				if t.upload.requests[slot].CompareAndSwap(nil, timedUpload) {
					break // successfully stored request.
				}
			}
		}
	}
}

func (t *Tracker) keepAliveLeechers(p *peer.Peer) {
	refresh := time.NewTicker(1 * time.Nanosecond) // first tick happens immediately.
	infoHash := string(t.Torrent.Metadata.Hash[:])
	for {
		select {
		case <-t.stop:
			t.logger.Debug("shutting down peer refresher, stopped tracker",
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
			t.upload.wg.Done()
			return
		case <-t.upload.cancel:
			t.logger.Debug("shutting down peer refresher, canceled upload",
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
			t.upload.wg.Done()
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch s := peer.ConnectionStatus(p.ConnectionStatus.Load()); s {
			case peer.ConnectionKilled, peer.ConnectionPending:
				t.logger.Info("shutting down peer refresher, connection closed",
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
				t.peers.leechers.Delete(p.Addr)
				t.upload.wg.Done()
				return
			case peer.ConnectionEstablished:
				t.logger.Info("sending keep alive event on torrent peer",
					slog.String("url", t.Torrent.Announce),
					slog.String("infoHash", infoHash),
					slog.String("peer_ip", p.Addr),
					slog.String("peer", p.Id),
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
