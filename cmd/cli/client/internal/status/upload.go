package status

import (
	"log/slog"
	"net"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer"
)

func (t *Tracker) CancelUpload() { close(t.upload.cancel); t.upload.wg.Wait() }

func (t *Tracker) AddLeecher(id string, conn net.Conn) error {
	//np := peer.NewLeecher(
	//	t.logger,
	//	id,
	//	conn.RemoteAddr().String(),
	//	t.Torrent.NumPieces(),
	//	conn,
	//)
	//
	//infoHash := string(t.Torrent.Metadata.Hash[:])
	//if err := np.SendHandshakeV1(infoHash, t.clientID); err != nil {
	//	return fmt.Errorf("failed to send handshake: %w", err)
	//}
	//
	//t.peers.leechers.Store(conn.RemoteAddr().String(), np)
	//
	//if err := np.SendBitfield(t.BitField.Clone()); err != nil {
	//	t.peers.leechers.Delete(conn.RemoteAddr().String())
	//	return fmt.Errorf("failed to send bitfield")
	//}
	//
	//t.upload.wg.Add(2)
	//go t.keepAliveLeechers(np)
	//go t.handleRequests(np)
	return nil
}

func (t *Tracker) processUploadRequests() {
	defer t.upload.wg.Done()
	currentRate := int64(0)
	rateTicker := time.NewTicker(rateTick)
	for {
		select {
		case <-t.stop:
			t.logger.Info("shutting down piece uploader, closed tracker")
			return
		case <-t.upload.cancel:
			t.logger.Info("shutting down piece uploader, canceled upload")
			return
		case <-rateTicker.C:
			newRate := t.Uploaded.Load()
			diff := max(0, newRate-currentRate)
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

func (t *Tracker) handleRequests(p *peer.Peer, requests <-chan *messagesv1.Request, cancels <-chan *messagesv1.Cancel) {
	logger := t.logger.With(slog.String("peer_ip", p.Addr), slog.String("pid", p.Id))
	for {
		select {
		case c, ok := <-cancels:
			if !ok {
				logger.Debug("shutting request handler, channel closed")
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
				logger.Debug("shutting request handler, channel closed")
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
	logger := t.logger.With(slog.String("peer_ip", p.Addr), slog.String("pid", p.Id))
	refresh := time.NewTicker(1 * time.Nanosecond) // first tick happens immediately.
	for {
		select {
		case <-t.stop:
			logger.Debug("shutting down peer refresher, stopped tracker")
			if err := p.Close(); err != nil {
				logger.Error("failed to close peer", slog.Any("err", err))
			}
			t.upload.wg.Done()
			return
		case <-t.upload.cancel:
			logger.Debug("shutting down peer refresher, canceled upload")
			if err := p.Close(); err != nil {
				logger.Error("failed to close peer", slog.Any("err", err.Error()))
			}
			t.upload.wg.Done()
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch s := p.ConnectionStatus(); s {
			// TODO: connection pending
			case peer.ConnectionKilled:
				logger.Info("shutting down peer refresher, connection closed")
				if err := p.Close(); err != nil {
					logger.Error("failed to close peer", slog.Any("err", err))
				}
				t.peers.leechers.Delete(p.Addr)
				t.upload.wg.Done()
				return
			case peer.ConnectionEstablished:
				logger.Info("sending keep alive event on torrent peer")
				if err := p.SendKeepAlive(); err != nil {
					logger.Error("failed to keep alive", slog.Any("err", err))
				}
			}
		}
	}
}
