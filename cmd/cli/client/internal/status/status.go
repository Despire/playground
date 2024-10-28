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

type timedRequest struct {
	request  messagesv1.Request
	send     time.Time
	received bool
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
	InFlight   []*timedRequest
}

type peers struct {
	seeders sync.Map
}

// How often the rate of bytes downloaded is updated.
const rateTick = 1 * time.Second

type Download struct {
	// Requests are the number of pieces concurrently
	// downloaded. No more than len(requests) pieces
	// are downloaded at a time.
	requests [14]atomic.Pointer[pendingPiece]
	// Download Related signaling. The downloadWg
	// is used when spawning download related goroutines.
	wg sync.WaitGroup
	// Download related signaling. When the torrent
	// finishes downloading the downloaded channel is
	// closed. Futher another API is exposed that
	// allows the application code to only cancel
	// the downloads and keep other workflows
	// running, such as seeding.
	cancel, completed chan struct{}
	// Rate is the number of bytes downloaded for the
	// last 10 seconds.
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

	// Stop channel indicates the the application was shutdown
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
		logger:      logger,
		stop:        make(chan struct{}),
		Torrent:     t,
		BitField:    bitfield.NewBitfield(t.NumBlocks()),
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
	return nil
}

func (t *Tracker) Flush(idx uint32, pieceBytes []byte) error {
	if _, err := os.Stat(t.DownloadDir); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(t.DownloadDir, os.ModePerm); err != nil {
			return err
		}
	}

	f, err := os.Create(path.Join(t.DownloadDir, fmt.Sprintf("%v.bin", idx)))
	if err != nil {
		return err
	}
	defer f.Close()

	return binary.Write(f, binary.LittleEndian, pieceBytes)
}
