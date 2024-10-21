package peer

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
)

//go:generate stringer -type=Status
type Status uint8

const (
	// Choked says whether the remote peer has choked this client.
	// When a peer chokes the client, it is a notification that
	// no requests will be answered until the client is unchoked.
	// The client should not attempt to send requests for blocks,
	// and it should consider all pending (unanswered) requests to
	// be discarded by the remote peer.
	Choked Status = iota
	// UnChoked says whether the remote peer is interested
	// in something this client has to offer. This is a notification
	// that the remote peer will begin requesting blocks when the client
	// unchokes them.
	UnChoked
)

//go:generate stringer -type=Interest
type Interest uint8

const (
	// Interested represents when remote peer is interested in something this client has to offer.
	// This is a notification that the remote peer will begin requesting blocks when the client unchokes them.
	Interested Interest = iota
	// NotInterested represents when a remote peer is not interested in something this client has to offer.
	NotInterested
)

//go:generate stringer -type=ConnectionStatus
type ConnectionStatus uint8

const (
	// ConnectionPending connection describes a state where a client
	// is waiting for the Peer to send the Handshake.
	ConnectionPending ConnectionStatus = iota
	// ConnectionEstablished connection describes a state where a peer
	// has sent the Handshake and both clients speak the same
	// protocol.
	ConnectionEstablished
	// ConnectionKilled connection describes a connection that was
	// terminated.
	ConnectionKilled
)

// Peer represents a peer in the swarm for sharing a file.
type Peer struct {
	logger *slog.Logger
	Id     string
	Addr   string

	conn             net.Conn
	ConnectionStatus ConnectionStatus

	Status struct {
		Remote Status
		This   Status
	}

	Interest struct {
		Remote Interest
		This   Interest
	}

	bitfield BitField
}

func New(logger *slog.Logger, id, addr string, pieces int64) (*Peer, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer at %s: %w", addr, err)
	}
	if id != "" {
		logger = logger.With(slog.String("peer_id", id))
	}

	p := &Peer{
		logger:           logger,
		Id:               id,
		Addr:             addr,
		conn:             conn,
		ConnectionStatus: ConnectionPending,
		bitfield:         make(BitField, (pieces/8)+1),
	}

	p.Status.Remote = Choked
	p.Status.This = Choked

	p.Interest.Remote = NotInterested
	p.Interest.This = NotInterested

	return p, nil
}

func (p *Peer) Close() error {
	if p.ConnectionStatus != ConnectionKilled {
		p.ConnectionStatus = ConnectionKilled
		return p.conn.Close()
	}
	return nil
}

// HandshakeV1 performs the handshake according to the version 1.0
// of the specifications. After a successful handshake a new goroutine
// is spawned that actively listens. on the established BitTorrent
// channel to decode incoming messages.
func (p *Peer) HandshakeV1(infoHash, peerID string) error {
	if p.ConnectionStatus == ConnectionEstablished {
		return nil
	}

	h := messagesv1.Handshake{
		Pstr:     messagesv1.ProtocolV1,
		Reserved: [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		InfoHash: infoHash,
		PeerID:   peerID,
	}

	msg := h.Serialize()

	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write v1 handshake message: %w", err)
	}

	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the v1 handshake message")
	}

	var resp [messagesv1.HandshakeLength]byte
	r, err := io.ReadFull(p.conn, resp[:])
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

	// adjust peer information.
	if p.Id == "" {
		p.Id = h.PeerID
		p.logger = p.logger.With(slog.String("peer_id", p.Id))
	}
	p.ConnectionStatus = ConnectionEstablished

	go p.listener()

	return nil
}

func (p *Peer) KeepAlive() error {
	if p.ConnectionStatus != ConnectionEstablished {
		return fmt.Errorf("invalid connection status %s, needed %s", p.ConnectionStatus, ConnectionEstablished)
	}

	msg := new(messagesv1.KeepAlive).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write keepalive message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the keepalive message")
	}

	return nil
}
