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
	np, err := peer.NewLeecherConnection(
		t.logger,
		id, conn.RemoteAddr().String(),
		t.Torrent.NumPieces(),
		conn,
		string(t.Torrent.Metadata.Hash[:]), t.clientID,
	)
	if err != nil {
		return fmt.Errorf("failed to establish leecher connection")
	}

	t.peers.leechers.Delete(conn.RemoteAddr().String())
	t.peers.leechers.Store(conn.RemoteAddr().String(), np)

	if err := np.SendBitfield(t.BitField.Clone()); err != nil {
		t.peers.leechers.Delete(conn.RemoteAddr().String())
		return fmt.Errorf("failed to send bitfield: %w", err)
	}

	r, c := np.Requests()

	t.upload.wg.Add(2)
	go t.keepAliveLeechers(np)
	go t.handleRequests(np, r, c)

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

						newUpload := t.Uploaded.Add(int64(len(b)))
						t.logger.Debug("uploaded piece",
							slog.String("piece", fmt.Sprint(req.request.Index)),
							slog.String("uploaded_bytes", fmt.Sprint(newUpload)),
						)
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

	defer func() {
		if err := p.Close(); err != nil {
			logger.Error("failed to close peer", slog.Any("err", err))
		}
		t.peers.leechers.Delete(p.Addr)
		t.upload.wg.Done()
	}()

	refresh := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-t.stop:
			logger.Debug("shutting down peer refresher, stopped tracker")
			return
		case <-t.upload.cancel:
			logger.Debug("shutting down peer refresher, canceled upload")
			return
		case <-refresh.C:
			refresh.Reset(2 * time.Minute)
			switch s := p.ConnectionStatus(); s {
			case peer.ConnectionKilled:
				logger.Info("shutting down peer refresher, connection closed")
				return
			case peer.ConnectionEstablished:
				logger.Info("sending keep alive event on torrent peer")
				if err := p.SendKeepAlive(); err != nil {
					logger.Error("failed to keep alive", slog.Any("err", err))
					return
				}
			}
		}
	}
}

func (t *Tracker) optimisticUnchoke() {
	defer t.upload.wg.Done()

	unchoke := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-t.stop:
			t.logger.Debug("shutting down peer refresher, stopped tracker")
			return
		case <-t.upload.cancel:
			t.logger.Debug("shutting down peer refresher, canceled upload")
			return
		case <-unchoke.C:
			// select random peer to unchoke
			t.peers.leechers.Range(func(_, value any) bool {
				p := value.(*peer.Peer)
				if p.Status.This.Load() == uint32(peer.Choked) && p.Interest.Remote.Load() == uint32(peer.Interested) {
					if err := p.SendUnchoke(); err != nil {
						t.logger.Error("failed to unchoke peer", slog.String("end_peer", p.Addr))
						return false
					}
				}
				return false
			})
		}
	}
}
