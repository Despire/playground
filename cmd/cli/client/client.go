package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/status"
	"github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/torrent"
)

var (
	// TorrentDir is the directory where the torrent files will
	// be downloaded.
	TorrentDir = os.Getenv("TORRENT_DIR")
)

func init() {
	if TorrentDir == "" {
		TorrentDir = "./tinytorrendDownloads"
		if _, err := os.Stat(TorrentDir); errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(TorrentDir, os.ModePerm); err != nil {
				panic(err)
			}
		}
	}
}

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

func (p *Client) Close() error { close(p.done); p.wg.Wait(); return nil }

func (p *Client) WorkOn(t *torrent.MetaInfoFile) (string, error) {
	h := string(t.Metadata.Hash[:])

	if _, ok := p.torrentsDownloading.Load(h); ok {
		return "", fmt.Errorf("torrent with hash %s is already tracked", h)
	}

	tr, err := status.NewTracker(p.id, p.logger, t, TorrentDir)
	if err != nil {
		return "", err
	}

	p.torrentsDownloading.Store(h, tr)

	p.handler <- h
	return h, nil
}

func (p *Client) WaitFor(id string) <-chan error {
	r := make(chan error, 1)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer close(r)
		s, ok := p.torrentsDownloading.Load(id)
		if !ok {
			r <- fmt.Errorf("torrent with id %s was not found, its possible that it was tracked but was deleted midway", id)
			return
		}

		tr := s.(*status.Tracker)
		for {
			select {
			case <-p.done:
				r <- errors.New("client shutting down")
				return
			case <-tr.WaitUntilDownloaded():
				switch {
				case tr.Torrent.InfoSingleFile != nil:
					final, err := os.Create(filepath.Join(tr.DownloadDir, tr.Torrent.InfoSingleFile.Name))
					if err != nil {
						r <- fmt.Errorf("failed to create torrent file for merging pieces: %w", err)
						break
					}

					pcs := tr.BitField.ExistingPieces()
					slices.Sort(pcs)

					q := tr.Torrent.NumPieces()
					_ = q

					copied := int64(0)
					var errAll error
					for _, i := range pcs {
						path := filepath.Join(tr.DownloadDir, fmt.Sprintf("%v.bin", i))
						f, err := os.Open(path)
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to open file for piece %v", i))
							continue
						}

						w, err := io.Copy(final, f)
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to copy piece %v to final merging file: %w", i, err))
							continue
						}

						copied += w
					}

					// TODO: add bandwith
					// TODO: Put a timer on the send requests received responses to avoid a deadlock.
					if copied != tr.Torrent.InfoSingleFile.Length {
						errAll = errors.Join(errAll, fmt.Errorf("failed to reconstruct torrent from downloaded pieces %d out of %d reconstructed", copied, tr.Torrent.InfoSingleFile.Length))
					}

					if errAll != nil {
						r <- fmt.Errorf("failed to reconstruct downloaded torrent: %w", errAll)
					}
				case tr.Torrent.InfoMultiFile != nil:
					// TODO: implement.
					r <- fmt.Errorf("multi file assembly not yet implemented")
				}
				return
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
			p.logger.Info("received signal to stop, stopping all torrents")

			p.torrentsDownloading.Range(func(key, value any) bool {
				id := key.(string)
				tr := value.(*status.Tracker)
				if err := tr.Close(); err != nil {
					p.logger.Error("failed to stop torrent", slog.String("torrent", id))
				}
				return true
			})
			p.torrentsDownloading.Clear()
			return
		}
	}
}

func (c *Client) downloadTorrent(ctx context.Context, infoHash string, t *status.Tracker) {
	const defaultPeerCount = 15

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

	if err := t.UpdateSeeders(start); err != nil {
		c.logger.Error("failed to update peers, attempting to continue",
			slog.String("err", err.Error()),
			slog.String("infoHash", infoHash),
		)
	}

	c.logger.Debug("entering update loop",
		slog.String("infoHash", infoHash),
		slog.String("url", t.Torrent.Announce),
	)

	ticker := time.NewTicker(time.Duration(*start.Interval) * time.Second)
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
					slog.String("url", t.Torrent.Announce),
				)
			}

			c.logger.Info("stopping download, context canceled",
				slog.String("url", t.Torrent.Announce),
				slog.String("infoHash", infoHash),
			)

			t.CancelDownload()
			c.wg.Done()
			return
		case <-t.WaitUntilDownloaded():
			c.logger.Info("sending completed update, finished downloaded torrent",
				slog.String("infoHash", infoHash),
				slog.String("url", t.Torrent.Announce),
			)
			_, err := tracker.CreateRequest(context.Background(), t.Torrent.Announce, &tracker.RequestParams{
				InfoHash:   infoHash,
				PeerID:     c.id,
				Port:       int64(c.port),
				Uploaded:   t.Uploaded.Load(),
				Downloaded: t.Downloaded.Load(),
				Left:       t.Torrent.BytesToDownload() - t.Downloaded.Load(),
				Compact:    tracker.Optional[int64](1),
				Event:      tracker.Optional(tracker.EventCompleted),
				TrackerID:  start.TrackerID,
			})
			if err != nil {
				c.logger.Error("failed announce completed event to tracker",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}
			t.CancelDownload()
			c.wg.Done()
			return
		case <-ticker.C:
			c.logger.Info("sending regular update based on interval",
				slog.String("infoHash", infoHash),
				slog.String("url", t.Torrent.Announce),
			)
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
					slog.String("url", t.Torrent.Announce),
				)
			}
			if event != nil {
				c.logger.Info("completed downloading torrent file",
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
				t.CancelDownload()
				c.wg.Done()
				return
			}
			if err := t.UpdateSeeders(update); err != nil {
				c.logger.Error("failed to update peers, attempting to continue",
					slog.String("err", err.Error()),
					slog.String("infoHash", infoHash),
					slog.String("url", t.Torrent.Announce),
				)
			}
		}
	}
}
