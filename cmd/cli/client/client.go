package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
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

type Action string

const (
	Leech = "leech"
	Both  = "both"
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
	action              Action
	seedServer          net.Listener

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

	if p.action != Leech {
		var err error
		if p.seedServer, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", p.port)); err != nil {
			return nil, fmt.Errorf("failed to announce listener server to the network: %w", err)
		}
		p.wg.Add(1)
		go p.acceptLeechers()
	}

	p.wg.Add(1)
	go p.watch()

	return p, nil
}

func (p *Client) Close() error {
	if p.seedServer != nil {
		p.seedServer.Close()
	}
	close(p.done)
	p.wg.Wait()

	p.torrentsDownloading.Range(func(key, value any) bool {
		id := key.(string)
		tr := value.(*status.Tracker)
		if err := tr.Close(); err != nil {
			p.logger.Error("failed to stop torrent", slog.String("torrent", id))
		}
		return true
	})
	p.torrentsDownloading.Clear()
	return nil
}

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

					copied := int64(0)
					var errAll error
					for _, i := range pcs {
						path := filepath.Join(tr.DownloadDir, fmt.Sprintf("%v.bin", i))
						f, err := os.Open(path)
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to open file for piece %v", i))
							continue
						}
						defer f.Close()

						w, err := io.Copy(final, f)
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to copy piece %v to final merging file: %w", i, err))
							continue
						}

						copied += w
					}

					if err := final.Close(); err != nil {
						errAll = errors.Join(errAll, err)
					}

					if copied != tr.Torrent.BytesToDownload() {
						errAll = errors.Join(errAll, fmt.Errorf("failed to reconstruct torrent from downloaded pieces %d out of %d reconstructed", copied, tr.Torrent.BytesToDownload()))
					}

					if errAll != nil {
						r <- fmt.Errorf("failed to reconstruct downloaded torrent: %w", errAll)
						break
					}
				case tr.Torrent.InfoMultiFile != nil:
					parent := filepath.Join(tr.DownloadDir, tr.Torrent.InfoMultiFile.Name)
					// create parent dir.
					if _, err := os.Stat(parent); errors.Is(err, os.ErrNotExist) {
						if err := os.Mkdir(parent, os.ModePerm); err != nil {
							r <- fmt.Errorf("failed to create parent directory for assembling multi file torrent")
							break
						}
					}

					// create torrent dir structure.
					var errAll error
					var files []io.WriteCloser
					for _, fi := range tr.Torrent.InfoMultiFile.Files {
						dir, filename := filepath.Split(fi.Path)
						if dir != "" {
							if err := os.MkdirAll(filepath.Join(parent, dir), os.ModePerm); err != nil {
								errAll = errors.Join(errAll, fmt.Errorf("failed to create parent dir for path %s: %w", fi.Path, err))
								continue
							}
						}

						file, err := os.Create(filepath.Join(parent, dir, filename))
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to create torrent file for merging pieces: %w", err))
							continue
						}
						defer file.Close()

						files = append(files, file)
					}

					if errAll != nil {
						r <- fmt.Errorf("failed to reconstruct multi-file torrent: %w", errAll)
						break
					}

					// prepare handles to pieces.
					pcs := tr.BitField.ExistingPieces()
					pieces := make([]io.ReadCloser, 0, len(pcs))

					for _, i := range pcs {
						path := filepath.Join(tr.DownloadDir, fmt.Sprintf("%v.bin", i))
						f, err := os.Open(path)
						if err != nil {
							errAll = errors.Join(errAll, fmt.Errorf("failed to open file for piece %v", i))
							continue
						}
						defer f.Close()
						pieces = append(pieces, f)

					}

					if errAll != nil {
						r <- fmt.Errorf("failed to reconstruct multi-file torrent: %w", errAll)
						break
					}

					tc, cp, l := int64(0), 0, tr.Torrent.PieceLength
					for i, fi := range tr.Torrent.InfoMultiFile.Files {
						for c := int64(0); c < fi.Length; {
							w, err := io.CopyN(files[i], pieces[cp], min(l, fi.Length-c))
							if err != nil {
								errAll = errors.Join(errAll, fmt.Errorf("failed to copy piece %v to final merging file %s: %w", i, fi.Path, err))
								continue
							}
							l -= w
							if l == 0 {
								cp++ // move to next piece.
								l = tr.Torrent.PieceLength
							}
							c += w
						}
						tc += fi.Length
					}

					if tc != tr.Torrent.BytesToDownload() {
						errAll = errors.Join(errAll, fmt.Errorf("failed to reconstruct torrent from downloaded pieces %d out of %d reconstructed", tc, tr.Torrent.BytesToDownload()))
					}

					if errAll != nil {
						r <- fmt.Errorf("failed to reconstruct multi-file torrent: %w", errAll)
						break
					}
				}
				return
			}
		}
	}()
	return r
}

func (p *Client) acceptLeechers() {
	defer p.wg.Done()
	for {
		conn, err := p.seedServer.Accept()
		if err != nil {
			p.logger.Error("failed to accept new incoming connections", slog.Any("err", err))
			if errors.Is(err, net.ErrClosed) {
				break
			}
		}

		p.wg.Add(1)
		go p.handlePeer(conn)
	}
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
			p.logger.Info("received signal to stop, issueing cancel to all torrents")
			cancel()
			return
		}
	}
}

func (c *Client) downloadTorrent(ctx context.Context, infoHash string, t *status.Tracker) {
	logger := c.logger.With(slog.String("url", t.Torrent.Announce), slog.String("infoHash", infoHash))
	const defaultPeerCount = 15

	var start *tracker.Response

tracker:
	for {
		select {
		case <-ctx.Done():
			c.wg.Done()
			return
		default:
			logger.Debug("initiating communication with tracker")

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
				logger.Error("failed to contact tracker", slog.Any("err", err))
				time.Sleep(10 * time.Second)
				continue
			}
			break tracker
		}
	}

	if start.Interval == nil {
		logger.Error("tracker did not returned announce interval, aborting.")
		c.wg.Done()
		return
	}

	logger.Info("received valid interval at which updates will be published to the tracker", slog.String("interval", fmt.Sprint(*start.Interval)))

	if err := t.UpdateSeeders(start); err != nil {
		logger.Error("failed to update peers, attempting to continue", slog.Any("err", err))
	}

	logger.Debug("entering update loop")

	ticker := time.NewTicker(time.Duration(*start.Interval) * time.Second)
	for {
		select {
		case <-ctx.Done():
			logger.Info("sending stop event on torrent")
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
				logger.Error("failed announce stop to tracker", slog.Any("err", err))
			}

			t.CancelDownload()
			c.wg.Done()

			logger.Info("stopping download, context canceled")
			return
		case <-t.WaitUntilDownloaded():
			logger.Info("sending completed update, finished downloaded torrent")
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
				logger.Error("failed announce completed event to tracker", slog.Any("err", err))
			}
			t.CancelDownload()
			c.wg.Done()
			logger.Info("download completed")
			return
		case <-ticker.C:
			logger.Info("sending regular update based on interval")
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
				logger.Error("failed announce regular update to tracker", slog.Any("err", err))
			}
			if event != nil {
				logger.Info("completed downloading torrent file")
				t.CancelDownload()
				c.wg.Done()
				return
			}
			if err := t.UpdateSeeders(update); err != nil {
				logger.Error("failed to update peers, attempting to continue", slog.Any("err", err))
			}
		}
	}
}
