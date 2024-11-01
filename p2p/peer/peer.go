package peer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/Despire/tinytorrent/p2p/peer/bitfield"
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
type ConnectionStatus int32

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

type peerType byte

const (
	leecher peerType = iota
	seeder
)

// Peer represents a peer in the swarm for sharing a file.
type Peer struct {
	logger *slog.Logger
	Id     string
	Addr   string

	wg               sync.WaitGroup
	conn             net.Conn
	ConnectionStatus atomic.Uint32
	typ              peerType

	Status struct {
		Remote atomic.Uint32
		This   atomic.Uint32
	}

	Interest struct {
		Remote atomic.Uint32
		This   atomic.Uint32
	}

	seeder struct {
		pieces chan *messagesv1.Piece
	}

	leecher struct {
		requests chan *messagesv1.Request
		cancels  chan *messagesv1.Cancel
	}

	Bitfield *bitfield.BitField
}

func NewSeeder(logger *slog.Logger, id, addr string, numPieces int64) *Peer {
	if id != "" {
		logger = logger.With(slog.String("peer_id", id))
	}

	p := &Peer{
		logger:   logger,
		Id:       id,
		Addr:     addr,
		conn:     nil,
		wg:       sync.WaitGroup{},
		Bitfield: bitfield.NewBitfield(numPieces),
		typ:      seeder,
	}

	p.seeder.pieces = make(chan *messagesv1.Piece)

	return p
}

func NewLeecher(logger *slog.Logger, id, addr string, numPieces int64, conn net.Conn) *Peer {
	p := &Peer{
		logger:   logger.With(slog.String("peer_id", id)),
		Id:       id,
		Addr:     addr,
		conn:     conn,
		wg:       sync.WaitGroup{},
		Bitfield: bitfield.NewBitfield(numPieces),
		typ:      leecher,
	}

	p.ConnectionStatus.Store(uint32(ConnectionEstablished))

	p.Status.Remote.Store(uint32(Choked))
	p.Status.This.Store(uint32(Choked))

	p.Interest.Remote.Store(uint32(NotInterested))
	p.Interest.This.Store(uint32(NotInterested))

	p.leecher.requests = make(chan *messagesv1.Request)
	p.leecher.cancels = make(chan *messagesv1.Cancel)

	p.wg.Add(1)
	go p.listener()

	return p
}

func (p *Peer) SeederPieces() <-chan *messagesv1.Piece { return p.seeder.pieces }

func (p *Peer) LeecherRequests() (<-chan *messagesv1.Request, <-chan *messagesv1.Cancel) {
	return p.leecher.requests, p.leecher.cancels
}

func (p *Peer) Close() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionKilled) {
		var err error
		if p.conn != nil {
			err = p.conn.Close()
		}
		p.ConnectionStatus.Store(uint32(ConnectionKilled))
		p.wg.Wait()
		return err
	}
	return nil
}

func (p *Peer) ConnectSeeder() error {
	if p.ConnectionStatus.Load() == uint32(ConnectionEstablished) {
		return errors.New("cannot connect on an already healthy connection")
	}

	var err error
	p.conn, err = net.DialTimeout("tcp", p.Addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to re-connect to peer at %s: %w", p.Addr, err)
	}

	p.ConnectionStatus.Store(uint32(ConnectionPending))

	p.Status.Remote.Store(uint32(Choked))
	p.Status.This.Store(uint32(Choked))

	p.Interest.Remote.Store(uint32(NotInterested))
	p.Interest.This.Store(uint32(NotInterested))

	p.seeder.pieces = make(chan *messagesv1.Piece)

	return nil
}

// InitiateHandshakeV1 performs the handshake according to the version 1.0
// of the specifications. After a successful handshake a new goroutine
// is spawned that actively listens. on the established BitTorrent
// channel to decode incoming messages.
func (p *Peer) InitiateHandshakeV1(infoHash, peerID string) error {
	if p.ConnectionStatus.Load() == uint32(ConnectionEstablished) {
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

	// adjust peer information.
	p.Id = h.PeerID
	p.logger = p.logger.With(slog.String("peer_id", p.Id))
	p.ConnectionStatus.Store(uint32(ConnectionEstablished))

	p.wg.Add(1)
	go p.listener()

	return nil
}

func (p *Peer) SendHandshakeV1(infoHash, peerID string) error {
	if p.ConnectionStatus.Load() == uint32(ConnectionEstablished) {
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

	return nil
}

func (p *Peer) SendKeepAlive() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
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

func (p *Peer) SendUnchoke() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := new(messagesv1.Unchoke).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write unchoke message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the unchoke message")
	}
	p.Status.This.Store(uint32(UnChoked))
	return nil
}

func (p *Peer) SendChoke() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := new(messagesv1.Choke).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write choke message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the choke message")
	}
	p.Status.This.Store(uint32(Choked))
	return nil
}

func (p *Peer) SendInterested() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := new(messagesv1.Interest).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write interest message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the interest message")
	}
	p.Interest.This.Store(uint32(Interested))
	return nil
}

func (p *Peer) SendNotInterested() error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := new(messagesv1.NotInterest).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write not-interest message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of the not-interest message")
	}
	p.Interest.This.Store(uint32(NotInterested))
	return nil
}

func (p *Peer) SendBitfield(b []byte) error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := (&messagesv1.Bitfield{Bitfield: b}).Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write bitfield message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of bitfield message")
	}
	return nil
}

func (p *Peer) SendRequest(req *messagesv1.Request) error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}

	msg := req.Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write request message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of request message")
	}
	return nil
}

func (p *Peer) SendCancel(cancel *messagesv1.Cancel) error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	if err := cancel.Validate(); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}

	msg := cancel.Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write request message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of request message")
	}
	return nil
}

func (p *Peer) SendHave(have *messagesv1.Have) error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := have.Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write have message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of have message")
	}
	return nil
}

func (p *Peer) SendPiece(piece *messagesv1.Piece) error {
	if p.ConnectionStatus.Load() != uint32(ConnectionEstablished) {
		return fmt.Errorf("invalid connection status %s, needed %s",
			ConnectionStatus(p.ConnectionStatus.Load()),
			ConnectionEstablished,
		)
	}

	msg := piece.Serialize()
	w, err := io.Copy(p.conn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to write have message: %w", err)
	}
	if int(w) != len(msg) {
		return fmt.Errorf("failed to write all of have message")
	}
	return nil
}
