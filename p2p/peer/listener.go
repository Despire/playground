package peer

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
)

// KeepAliveTimeout represents the maximum timeout for recieving a
// keep alive message. Once passed the connection will be terminated.
const KeepAliveTimeout = 3 * time.Minute

func (p *Peer) listener() {
	for p.ConnectionStatus.Load() == uint32(ConnectionEstablished) {
		if err := p.conn.SetReadDeadline(time.Now().Add(KeepAliveTimeout)); err != nil {
			p.logger.Error("failed to set read deadline to KeepAliveTimeout",
				slog.String("err", err.Error()),
			)
			continue
		}

		msg, err := messagesv1.Identify(p.conn)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				p.logger.Error("peer read exceeded KeepAliveTimeout Closing connection.")
				break
			}
			if errors.Is(err, io.EOF) {
				p.logger.Error("peer read EOF reading from connection.")
				break
			}
			p.logger.Error("failed to read from connection",
				slog.String("err", err.Error()),
			)
			continue
		}

		p.logger.Debug("received message type", slog.String("type", msg.Type.String()))
		if err := p.process(msg); err != nil {
			p.logger.Error("failed to process message",
				slog.String("type", msg.Type.String()),
				slog.String("err", err.Error()),
			)
		}
	}

	p.ConnectionStatus.Store(uint32(ConnectionKilled))
	close(p.pieces)
	p.wg.Done()
	if err := p.conn.Close(); err != nil {
		p.logger.Error("failed to cose connection", slog.String("err", err.Error()))
	}

	p.logger.Debug("peer connection shutting down")
}

func (p *Peer) process(msg *messagesv1.Message) error {
	// TODO: determine when to close connection.
	switch msg.Type {
	case messagesv1.KeepAliveType:
		// do nothing.
		return nil
	case messagesv1.ChokeType: // receive choked from remote peer.
		p.Status.Remote.Store(uint32(Choked))
		return nil
	case messagesv1.UnChokeType: // receive unchoke from remote peer.
		p.Status.Remote.Store(uint32(UnChoked))
		return nil
	case messagesv1.InterestType: // recieve interest from remote peer.
		p.Interest.Remote.Store(uint32(Interested))
		return nil
	case messagesv1.NotInterestType: // receive notinterest from remote peer.
		p.Interest.Remote.Store(uint32(NotInterested))
		return nil
	case messagesv1.HaveType: // peer announce that he completed donwloading piecie with index.
		h := new(messagesv1.Have)
		if err := h.Deserialize(msg.Payload); err != nil {
			return fmt.Errorf("could not deserialize message %s: %w", msg.Type, err)
		}
		if err := p.Bitfield.SetWithCheck(h.Index); err != nil {
			return fmt.Errorf("could not acknowledge piece %v: %w", h.Index, err)
		}
		p.logger.Debug("updated bitfield based on have message")
		return nil
	case messagesv1.BitfieldType: // peer send what pieces he possesses.
		b := new(messagesv1.Bitfield)
		if err := b.Deserialize(msg.Payload); err != nil {
			return fmt.Errorf("could not deserialize message %s: %w", msg.Type, err)
		}

		if len(b.Bitfield) != p.Bitfield.Len() {
			return errors.New("received incorrect bit-flied length")
		}
		p.Bitfield.Overwrite(b.Bitfield)
		p.logger.Debug("updated bitfield based on bitfield message")
		return nil
	case messagesv1.PieceType: // peer send a piece
		pc := new(messagesv1.Piece)

		if err := pc.Deserialize(msg.Payload); err != nil {
			return fmt.Errorf("could not deserialize message %s: %w", msg.Type, err)
		}
		p.pieces <- pc
		return nil
	case messagesv1.PortType: // peer requested DHT extension.
		return fmt.Errorf("dht is not supported")
	case messagesv1.RequestType:
		return fmt.Errorf("did not expect a request message on a leech connection")
	case messagesv1.CancelType:
		return fmt.Errorf("did not expect a cancel message on a leech connection")
	default:
		return fmt.Errorf("no implementation for processing message type: %s", msg.Type)
	}
}
