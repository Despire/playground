package status

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer/bitfield"
	"github.com/Despire/tinytorrent/torrent"
)

type timedDownloadRequest struct {
	request  messagesv1.Request
	send     time.Time
	received bool
}

type timedUploadRequest struct {
	request  messagesv1.Request
	recieved time.Time
	addr     string
}

type pendingPiece struct {
	// l guards against concurrent accesses
	// for the fields. Useful to have
	// a consistent snapshot.
	l          sync.Mutex
	Index      uint32
	Downloaded int64
	Size       int64
	Received   []*messagesv1.Piece
	Pending    []*messagesv1.Request
	InFlight   []*timedDownloadRequest
}

type peers struct {
	seeders  sync.Map
	leechers sync.Map
}

// How often the rate of bytes downloaded is updated.
const rateTick = 1 * time.Second

type Download struct {
	// Requests are the number of pieces concurrently
	// downloaded. No more than len(requests) pieces
	// are downloaded at a time.
	requests [10]atomic.Pointer[pendingPiece]
	// The wait group is used when spawning download related goroutines.
	wg sync.WaitGroup
	// Download related signaling. When the torrent
	// finishes downloading the downloaded channel is
	// closed. Futher another API is exposed that
	// allows the application code to only cancel
	// the downloads and keep other workflows
	// running, such as seeding.
	cancel, completed chan struct{}
	// Rate is the number of bytes downloaded for the last 1 seconds.
	rate atomic.Int64
}

type Upload struct {
	// Requests are the number of maximum requests
	// that will be handled by the client for any
	// number of connected leechers.
	requests [20]atomic.Pointer[timedUploadRequest]
	// The wait group is used when spawning upload related goroutines.
	wg sync.WaitGroup
	// Upload related signaling. When the torrent
	// finishes uploading the cancel channel is closed.
	cancel chan struct{}
	// Rate is the number of bytes uploaded for the last 1 second.
	rate atomic.Int64
}

// Tracker wraps all necessary information for tracking
// the status of a torrent file
type Tracker struct {
	clientID string
	logger   *slog.Logger

	// Peers are the seeders and leechers that are known
	// to this torrent tracker.
	peers peers

	// download wraps all download related information.
	download Download

	// upload wraps all upload related information.
	upload Upload

	// Stop channel indicates the application was shutdown
	// By closing this channel all workflows will finish
	// and the tracker will no longer do any work.
	stop chan struct{}

	Torrent     *torrent.MetaInfoFile
	BitField    *bitfield.BitField
	Uploaded    atomic.Int64
	Downloaded  atomic.Int64
	DownloadDir string
}

func NewTracker(clientID string, logger *slog.Logger, t *torrent.MetaInfoFile, downloadDir string) (*Tracker, error) {
	tr := Tracker{
		clientID:    clientID,
		logger:      logger.With(slog.String("url", t.Announce), slog.String("infoHash", string(t.Metadata.Hash[:]))),
		stop:        make(chan struct{}),
		Torrent:     t,
		BitField:    bitfield.NewBitfield(t.NumPieces()),
		Uploaded:    atomic.Int64{},
		Downloaded:  atomic.Int64{},
		DownloadDir: path.Join(downloadDir, hex.EncodeToString(t.Info.Metadata.Hash[:])),
	}

	tr.download.cancel = make(chan struct{})
	tr.download.completed = make(chan struct{})

	// read bitfield if exists.
	f, err := os.Open(filepath.Join(tr.DownloadDir, "bitfield.bin"))
	if err == nil {
		defer f.Close()
		r := make([]byte, tr.BitField.Len())
		if err := binary.Read(f, binary.LittleEndian, &r); err != nil {
			return nil, fmt.Errorf("failed to read existing bitfield file: %w", err)
		}

		tr.BitField.Overwrite(r)

		// calculated downloaded size.
		for _, i := range tr.BitField.ExistingPieces() {
			pieceStart := int64(i) * tr.Torrent.PieceLength
			pieceEnd := pieceStart + tr.Torrent.PieceLength
			pieceEnd = min(pieceEnd, tr.Torrent.BytesToDownload())
			tr.Downloaded.Add(pieceEnd - pieceStart)
		}
	}

	tr.download.wg.Add(1)
	go tr.downloadScheduler()

	tr.upload.wg.Add(1)
	go tr.processUploadRequests()
	return &tr, nil
}

func (t *Tracker) Close() error {
	var errAll error
	// write bitfiled to file
	b, err := os.Create(filepath.Join(t.DownloadDir, "bitfield.bin"))
	if err != nil {
		errAll = errors.Join(errAll, fmt.Errorf("failed creating bitfiled file: %w", err))
	} else {
		if err := binary.Write(b, binary.LittleEndian, t.BitField.Clone()); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("failed to write bitfield to disk: %w", err))
		}
		if err := b.Close(); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("failed to close bitfield file: %w", err))
		}
	}
	close(t.stop)
	t.download.wg.Wait()
	t.upload.wg.Wait()
	return nil
}

func (t *Tracker) Flush(idx uint32, pieceBytes []byte) error {
	if _, err := os.Stat(t.DownloadDir); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(t.DownloadDir, os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.Create(filepath.Join(t.DownloadDir, fmt.Sprintf("%v.bin", idx)))
	if err != nil {
		return err
	}
	defer f.Close()

	return binary.Write(f, binary.LittleEndian, pieceBytes)
}

func (t *Tracker) ReadRequest(req *messagesv1.Request) ([]byte, error) {
	if _, err := os.Stat(t.DownloadDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cannot construct request: %w", err)
	}

	b, err := os.ReadFile(filepath.Join(t.DownloadDir, fmt.Sprintf("%v.bin", req.Index)))
	if err != nil {
		return nil, err
	}

	if int(req.Begin) >= len(b) {
		return nil, fmt.Errorf("invalid request, offset within piece larger than piece size")
	}

	if l := len(b) - int(req.Begin); int(req.Length) > l {
		return nil, fmt.Errorf("invalid request, offset + length tries to request larger block than possible")
	}

	return b[int(req.Begin):(int(req.Begin + req.Length))], nil
}
