package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/Despire/tinytorrent/p2p"
	"github.com/Despire/tinytorrent/p2p/client/internal/status"
	"github.com/Despire/tinytorrent/p2p/client/internal/tracker"
	"github.com/Despire/tinytorrent/torrent"
)

// Client represents a single instance of a peer within
// the BitTorrent network.
type Client struct {
	id     string
	logger *slog.Logger
	port   int

	handler chan string
	done    chan struct{}

	torrents map[string]*status.Tracker
}

func New(opts ...Option) (*Client, error) {
	p := &Client{
		handler:  make(chan string),
		done:     make(chan struct{}),
		torrents: make(map[string]*status.Tracker),
	}
	defaults(p)

	for _, o := range opts {
		o(p)
	}

	go p.watch()

	return p, nil
}

func (p *Client) Close() error { close(p.done); return nil }

func (p *Client) WorkOn(t *torrent.MetaInfoFile) (string, error) {
	h := string(t.Metadata.Hash[:])
	if _, ok := p.torrents[h]; ok {
		return "", fmt.Errorf("torrent with hash %s is already tracked", h)
	}

	p.torrents[h] = &status.Tracker{
		Torrent:    t,
		Uploaded:   atomic.Int64{},
		Downloaded: atomic.Int64{},
	}

	p.handler <- h
	return h, nil
}

func (p *Client) WaitFor(id string) <-chan error {
	r := make(chan error)
	go func() {
		defer close(r)
		for {
			select {
			case <-p.done:
				r <- errors.New("peer closed")
				return
			default:
				s, ok := p.torrents[id]
				if !ok {
					r <- fmt.Errorf("torrent with id %s was not found, its possible that it was tracked but was deleted midway", id)
					return
				}

				if s.Torrent.BytesToDownload() == s.Downloaded.Load() {
					return
				}

				time.Sleep(10 * time.Second)
			}
		}
	}()
	return r
}

func (p *Client) watch() {
	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case infoHash := <-p.handler:
			go p.handle(ctx, infoHash, p.torrents[infoHash])
		case <-p.done:
			cancel()
			p.logger.Info("received signal to stop")
			return
		}
	}
}

func (c *Client) handle(ctx context.Context, infoHash string, t *status.Tracker) {
	const defaultPeerCount = 30

	c.logger.Debug("initiating communication with tracker",
		slog.String("url", t.Torrent.Announce),
		slog.String("infoHash", infoHash),
	)

	start, err := tracker.CreateRequest(ctx, t.Torrent.Announce, &tracker.RequestParams{
		InfoHash:   infoHash,
		PeerID:     c.id,
		Port:       int64(c.port),
		Uploaded:   0,
		Downloaded: 0,
		Left:       t.Torrent.BytesToDownload(),
		Compact:    tracker.Optional[int64](1),
		Event:      tracker.Optional(tracker.EventStarted),
		NumWant:    tracker.Optional[int64](defaultPeerCount),
	})
	if err != nil {
		c.logger.Error("failed to contact tracker",
			slog.String("err", err.Error()),
			slog.String("infoHash", infoHash),
		)
		return
	}

	if start.Interval == nil {
		c.logger.Error("tracker did not returned announce interval, aborting.",
			slog.String("infoHash", infoHash),
		)
		return
	}

	c.logger.Info("received valid interval at which updates will be published to the tracker",
		slog.String("infoHash", infoHash),
		slog.String("interval", fmt.Sprint(*start.Interval)),
	)

	c.logger.Info("initiating handshake with peers",
		slog.String("infoHash", infoHash),
		slog.String("peers", fmt.Sprint(start.Peers)),
	)

	peers := make(map[string]*p2p.Peer, len(start.Peers))
	if err := updatePeers(peers, start, infoHash, c.id); err != nil {
		c.logger.Error("failed to update peers, attempting to continue",
			slog.String("err", err.Error()),
			slog.String("infoHash", infoHash),
		)
	}

	// TODO: download blocks...

	ticker := time.NewTicker(time.Duration(*start.Interval))
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("sending stop event on torrent",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			_, err := tracker.CreateRequest(context.Background(), t.Torrent.Announce, &tracker.RequestParams{
				InfoHash:   infoHash,
				PeerID:     c.id,
				Port:       int64(c.port),
				Uploaded:   t.Uploaded.Load(),
				Downloaded: t.Downloaded.Load(),
				Left:       t.Torrent.BytesToDownload() - t.Downloaded.Load(),
				Compact:    tracker.Optional[int64](1),
				Event:      tracker.Optional(tracker.EventStopped),
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				c.logger.Error("failed announce stop to tracker",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
				)
			}
			return
		case <-ticker.C:
			var event *tracker.Event
			if completed := (t.Torrent.BytesToDownload() - t.Downloaded.Load()) == 0; completed {
				event = tracker.Optional(tracker.EventCompleted)
			}
			update, err := tracker.CreateRequest(context.Background(), t.Torrent.Announce, &tracker.RequestParams{
				InfoHash:   infoHash,
				PeerID:     c.id,
				Port:       int64(c.port),
				Uploaded:   t.Uploaded.Load(),
				Downloaded: t.Downloaded.Load(),
				Left:       t.Torrent.BytesToDownload() - t.Downloaded.Load(),
				Compact:    tracker.Optional[int64](1),
				Event:      event,
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				c.logger.Error("failed announce regular update to tracker",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
				)
			}
			if event != nil {
				c.logger.Info("completed downloading torrent file",
					slog.String("infoHash", infoHash),
				)
				return
			}

			c.logger.Info("updating peers based on regular interval",
				slog.String("infoHash", infoHash),
				slog.String("peers", fmt.Sprint(start.Peers)),
			)

			if err := updatePeers(peers, update, infoHash, c.id); err != nil {
				c.logger.Error("failed to update peers, attempting to continue",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
				)
			}
		}
	}
}

func updatePeers(peers map[string]*p2p.Peer, resp *tracker.Response, infoHash, clientID string) error {
	var errAll error

	for _, r := range resp.Peers {
		ip := net.JoinHostPort(r.IP, fmt.Sprint(r.Port))
		p, ok := peers[ip]
		if !ok {
			np, err := p2p.NewPeer(r.PeerID, ip)
			if err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to connect to peer %s: %w", ip, err))
				continue
			}

			peers[ip] = np
			p = np
		}

		switch p.ConnectionStatus {
		case p2p.Pending:
			if err := p.HandshakeV1(infoHash, clientID); err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to handshake with peer %s: %w", ip, err))
			}
		case p2p.Killed:
			delete(peers, ip)
		}
	}

	return errAll
}
