package status

import (
	"sync/atomic"

	"github.com/Despire/tinytorrent/torrent"
)

// Tracker wraps all necessary information for tracking
// the status of a torrent file
type Tracker struct {
	// Read Only.
	Torrent    *torrent.MetaInfoFile
	Uploaded   atomic.Int64
	Downloaded atomic.Int64
}
