package client

import (
	"crypto/rand"
	"log/slog"
	"os"
	"testing"

	"github.com/Despire/tinytorrent/p2p/peer"
)

func TestAddNewPeer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	var id [20]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatalf("failed to generate peer id: %v", err)
		return
	}

	peerAddr := "127.0.0.1:6882"

	p := peer.New(logger, "", peerAddr, 2105)

	if err := p.Connect(); err != nil {
		t.Fatalf("failed to connect to peer: %v", err)
		return
	}

	if err := p.InitiateHandshakeV1("\xe3{d\xd8\\\xf4\xaa\x93\xe0\xecJ\xee+Ds[|\xb69g", string(id[:])); err != nil {
		t.Fatalf("failed to handshake with peer: %v", err)
		return
	}

	select {}
}
