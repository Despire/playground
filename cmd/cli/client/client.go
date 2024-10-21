package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/status"
	"github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/p2p/peer"
	"github.com/Despire/tinytorrent/torrent"
)

// Client represents a single instance of a peer within
// the BitTorrent network.
type Client struct {
	id   string
	port int

	logger *slog.Logger

	handler chan string
	done    chan struct{}

	torrentsDownloading sync.Map

	wg sync.WaitGroup
}

func New(opts ...Option) (*Client, error) {
	p := &Client{
		handler: make(chan string),
		done:    make(chan struct{}),
	}
	defaults(p)

	for _, o := range opts {
		o(p)
	}

	p.wg.Add(1)
	go p.watch()

	return p, nil
}

func (p *Client) Close() error { close(p.done); return nil }

func (p *Client) WorkOn(t *torrent.MetaInfoFile) (string, error) {
	h := string(t.Metadata.Hash[:])

	if _, ok := p.torrentsDownloading.Load(h); ok {
		return "", fmt.Errorf("torrent with hash %s is already tracked", h)
	}

	p.torrentsDownloading.Swap(h, &status.Tracker{
		Torrent:    t,
		Uploaded:   0,
		Downloaded: 0,
		Peers:      make(map[string]*peer.Peer),
	})

	p.handler <- h
	return h, nil
}

func (p *Client) WaitFor(id string) <-chan error {
	r := make(chan error)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer close(r)
		for {
			select {
			case <-p.done:
				r <- errors.New("client shutting down")
				return
			default:
				s, ok := p.torrentsDownloading.Load(id)
				if !ok {
					r <- fmt.Errorf("torrent with id %s was not found, its possible that it was tracked but was deleted midway", id)
					return
				}

				p := s.(*status.Tracker)
				if p.Torrent.BytesToDownload() == p.Downloaded {
					return
				}

				time.Sleep(10 * time.Second)
			}
		}
	}()
	return r
}

func (p *Client) watch() {
	defer p.wg.Done()
	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case infoHash := <-p.handler:
			t, _ := p.torrentsDownloading.Load(infoHash)
			p.wg.Add(1)
			go p.downloadTorrent(ctx, infoHash, t.(*status.Tracker))
		case <-p.done:
			cancel()
			p.logger.Info("received signal to stop, waiting for all goroutines to finish")
			p.wg.Wait()
			return
		}
	}
}

func (c *Client) downloadTorrent(ctx context.Context, infoHash string, t *status.Tracker) {
	const defaultPeerCount = 30

	var start *tracker.Response

tracker:
	for {
		select {
		case <-ctx.Done():
			c.wg.Done()
			return
		default:
			c.logger.Debug("initiating communication with tracker",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)

			var err error
			start, err = tracker.CreateRequest(ctx, t.Torrent.Announce, &tracker.RequestParams{
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
				time.Sleep(10 * time.Second)
				continue
			}
			break tracker
		}
	}

	if start.Interval == nil {
		c.logger.Error("tracker did not returned announce interval, aborting.",
			slog.String("infoHash", infoHash),
		)
		c.wg.Done()
		return
	}

	c.logger.Info("received valid interval at which updates will be published to the tracker",
		slog.String("infoHash", infoHash),
		slog.String("interval", fmt.Sprint(*start.Interval)),
	)

	if err := t.UpdatePeers(c.id, c.logger, start); err != nil {
		c.logger.Error("failed to update peers, attempting to continue",
			slog.String("err", err.Error()),
			slog.String("infoHash", infoHash),
		)
	}

	// TODO: download blocks...
	c.logger.Debug("entering update loop",
		slog.String("infoHash", infoHash),
		slog.String("url", t.Torrent.Announce),
	)

	ticker := time.NewTicker(time.Duration(*start.Interval) * time.Second)
	keepAlive := time.NewTicker(2 * time.Minute)
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
				Uploaded:   t.Uploaded,
				Downloaded: t.Downloaded,
				Left:       t.Torrent.BytesToDownload() - t.Downloaded,
				Compact:    tracker.Optional[int64](1),
				Event:      tracker.Optional(tracker.EventStopped),
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				c.logger.Error("failed announce stop to tracker",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}

			c.logger.Info("closing peers",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)

			if err := t.Stop(); err != nil {
				c.logger.Info("failed to close connections to some of the peers.",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}

			c.wg.Done()
			return
		case <-keepAlive.C:
			c.logger.Info("sending keep alive event on torrent peers",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)
			for _, p := range t.Peers {
				if err := p.KeepAlive(); err != nil {
					c.logger.Error("failed to keep alive",
						slog.String("err", err.Error()),
						slog.String("infoHash", infoHash),
						slog.String("url", t.Torrent.Announce),
						slog.String("peer", p.Id),
					)
				}
			}
		case <-ticker.C:
			c.logger.Info("sending regular update based on interval",
				slog.String("infoHash", infoHash),
				slog.String("url", t.Torrent.Announce),
				slog.String("peers", fmt.Sprint(start.Peers)),
			)
			var event *tracker.Event
			if completed := (t.Torrent.BytesToDownload() - t.Downloaded) == 0; completed {
				event = tracker.Optional(tracker.EventCompleted)
			}
			update, err := tracker.CreateRequest(context.Background(), t.Torrent.Announce, &tracker.RequestParams{
				InfoHash:   infoHash,
				PeerID:     c.id,
				Port:       int64(c.port),
				Uploaded:   t.Uploaded,
				Downloaded: t.Downloaded,
				Left:       t.Torrent.BytesToDownload() - t.Downloaded,
				Compact:    tracker.Optional[int64](1),
				Event:      event,
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				c.logger.Error("failed announce regular update to tracker",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}
			if event != nil {
				c.logger.Info("completed downloading torrent file",
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
				return
			}
			if err := t.UpdatePeers(c.id, c.logger, update); err != nil {
				c.logger.Error("failed to update peers, attempting to continue",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}
		}
	}
}
