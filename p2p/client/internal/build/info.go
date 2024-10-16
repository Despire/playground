package build

import (
	"fmt"
	"runtime"
	"time"
)

// ClientID identifies this torrent client.
const ClientID = "MM"

var (
	// Date is set via ldflags on build.
	// Must have the format time.DateOnly
	Date string

	// Hash is set via ldflags on build.
	// Can be arbitrary long.
	Hash string

	// Version is set via ldflags on build.
	// Version is expected to be exactly 4 bytes long.
	Version string
)

// Info  wraps relevant information with
// which the client was build.
type Info struct {
	ClientID      string
	ClientVersion string
	BuildDate     time.Time
	BuildHash     string
	GoVersion     string
	Compiler      string
	Platform      string
	Architecture  string
}

func Information() Info {
	t, err := time.Parse(time.DateOnly, Date)
	if err != nil {
		panic(fmt.Sprintf("failed to parse build.Date: %v", err))
	}

	return Info{
		ClientID:      ClientID,
		ClientVersion: Version,
		BuildDate:     t,
		BuildHash:     Hash,
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      runtime.GOOS,
		Architecture:  runtime.GOARCH,
	}
}
