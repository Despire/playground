package p2p

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
)

type PeerStatus string

const (
	// Choked says whether the remote peer has choked this client.
	// When a peer chokes the client, it is a notification that
	// no requests will be answered until the client is unchoked.
	// The client should not attempt to send requests for blocks,
	// and it should consider all pending (unanswered) requests to
	// be discarded by the remote peer.
	Choked PeerStatus = "choked"
	// UnChoked says whether the remote peer is interested
	// in something this client has to offer. This is a notification
	// that the remote peer will begin requesting blocks when the client
	// unchokes them.
	UnChoked PeerStatus = "unchoked"
)

type ConnectionStatus string

const (
	// Pending connection describes a state where a client
	// is waiting for the Peer to send the Handshake.
	Pending ConnectionStatus = "new"
	// Established connection describes a state where a peer
	// has sent the Handshake and both clients speak the same
	// protocol.
	Established ConnectionStatus = "established"
	// Killed connection describes a connection that was
	// terminated.
	Killed ConnectionStatus = "killed"
)

// Peer represents a peer in the swarm for sharing a file.
type Peer struct {
	Id               string
	Addr             string
	Conn             net.Conn
	ConnectionStatus ConnectionStatus
	// Peer status related to this client.
	PeerStatus PeerStatus
	// This Clients status related to this Peer.
	ClientStatus PeerStatus
}

func NewPeer(id string, addr string) (*Peer, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer at %s: %w", addr, err)
	}

	return &Peer{
		Id:               id,
		Addr:             addr,
		Conn:             conn,
		ConnectionStatus: Pending,
		PeerStatus:       Choked,
		ClientStatus:     Choked,
	}, nil
}

func (p *Peer) Close() error { p.ConnectionStatus = Killed; return p.Conn.Close() }

// HandshakeV1 performs the handshake according to the version 1.0
// of the specifications.
func (p *Peer) HandshakeV1(infoHash, peerID string) error {
	if p.ConnectionStatus == Established {
		return nil
	}

	h := messagesv1.Handshake{
		Pstr:     messagesv1.ProtocolV1,
		Reserved: [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		InfoHash: infoHash,
		PeerID:   peerID,
	}

	msg := h.Serialize()

	w, err := io.Copy(p.Conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write v1 handshake message: %w", err)
	}

	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the v1 handshake message")
	}

	var resp [messagesv1.HandshakeLength]byte
	r, err := io.ReadFull(p.Conn, resp[:])
	if err != nil {
		return fmt.Errorf("failed to read v1 handshake message: %w", err)
	}

	if r != len(resp) {
		return fmt.Errorf("only partial handshake message received")
	}

	if err := h.Deserialize(resp[:]); err != nil {
		return fmt.Errorf("failed to deserialize v1 handshake message: %w", err)
	}

	if err := h.Validate(); err != nil {
		return fmt.Errorf("failed to validate v1 handshake message: %w", err)
	}

	if p.Id != "" && h.PeerID != p.Id {
		return fmt.Errorf("invalid v1 handshake message peer id mismatch")
	}

	p.ConnectionStatus = Established

	return nil
}
