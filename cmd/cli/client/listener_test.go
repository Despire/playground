package client

import (
	"bytes"
	"log/slog"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/status"
	tracker2 "github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/torrent"
	"github.com/stretchr/testify/assert"
)

func TestAddNewPeer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	err := os.Mkdir("./testDownload", os.ModePerm)
	assert.Nil(t, err)

	t.Cleanup(func() { os.RemoveAll("./testDownload") })

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}))

	var id [20]byte
	if _, err := new(rand.ChaCha8).Read(id[:]); err != nil {
		t.Fatalf("failed to generate peer id: %v", err)
		return
	}

	peerAddr := "127.0.0.1"
	port := int64(6882)

	b, err := os.ReadFile("../../../torrent/test_data/debian.torrent")
	assert.Nil(t, err)
	tr, err := torrent.From(bytes.NewReader(b))
	assert.Nil(t, err)

	resp := tracker2.Response{}
	resp.Peers = append(resp.Peers, struct {
		PeerID string
		IP     string
		Port   int64
	}{PeerID: "", IP: peerAddr, Port: port})

	tracker, err := status.NewTracker(string(id[:]), logger, tr, "./testDownload")
	assert.Nil(t, err)

	err = tracker.UpdateSeeders(&resp)
	assert.Nil(t, err)

	<-tracker.WaitUntilDownloaded()
}
