package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Despire/tinytorrent/p2p/client/internal/status"
	"github.com/Despire/tinytorrent/p2p/client/internal/tracker"
	"github.com/Despire/tinytorrent/torrent"
)

// Peer represents a single instance of a peer within
// the BitTorrent network.
type Peer struct {
	id     string
	logger *slog.Logger
	port   int

	handler  chan string
	done     chan struct{}
	torrents map[string]*status.Tracker
}

func New(opts ...Option) (*Peer, error) {
	p := &Peer{
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

func (p *Peer) Close() error { close(p.done); return nil }

func (p *Peer) WorkOn(t *torrent.MetaInfoFile) (string, error) {
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

func (p *Peer) WaitFor(id string) <-chan error {
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

func (p *Peer) watch() {
	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case id := <-p.handler:
			go p.handle(ctx, id, p.torrents[id])
		case <-p.done:
			cancel()
			p.logger.Info("received signal to stop")
			return
		}
	}
}

func (p *Peer) handle(ctx context.Context, id string, t *status.Tracker) {
	const defaultPeerCount = 30

	p.logger.Debug("initiating communication with tracker", "url", t.Torrent.Announce)

	start, err := tracker.CreateRequest(ctx, t.Torrent.AnnounceList[1], &tracker.RequestParams{
		InfoHash:   id,
		PeerID:     p.id,
		Port:       int64(p.port),
		Uploaded:   0,
		Downloaded: 0,
		Left:       t.Torrent.BytesToDownload(),
		Compact:    tracker.SetOptional[int64](1),
		Event:      tracker.SetOptional(tracker.EventStarted),
		NumWant:    tracker.SetOptional[int64](defaultPeerCount),
	})
	if err != nil {
		p.logger.Error("failed to contact tracker", "err", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("stopping work on torrent: %s, received signal stop", slog.String("url", t.Torrent.Announce))
			_, err := tracker.CreateRequest(context.Background(), t.Torrent.Announce, &tracker.RequestParams{
				InfoHash:   id,
				PeerID:     p.id,
				Port:       int64(p.port),
				Uploaded:   t.Uploaded.Load(),
				Downloaded: t.Downloaded.Load(),
				Left:       t.Torrent.BytesToDownload() - t.Downloaded.Load(),
				Compact:    tracker.SetOptional[int64](1),
				Event:      tracker.SetOptional(tracker.EventStopped),
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				p.logger.Error("failed announce stop to tracker", "err", err)
			}
			return
		}
	}
}
