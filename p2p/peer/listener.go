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
	for p.ConnectionStatus == ConnectionEstablished {
		if err := p.conn.SetReadDeadline(time.Now().Add(KeepAliveTimeout)); err != nil {
			p.logger.Info("failed to set read deadline to KeepAliveTimeout",
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

		p.logger.Info("received message type", slog.String("type", msg.Type.String()))
		if err := p.process(msg); err != nil {
			p.logger.Error("failed to process message",
				slog.String("type", msg.Type.String()),
				slog.String("err", err.Error()),
			)
		}
	}

	p.logger.Info("peer connection shutting down")

	if err := p.Close(); err != nil {
		p.logger.Info("failed to close connection", slog.String("err", err.Error()))
	}
}

func (p *Peer) process(msg *messagesv1.Message) error {
	switch msg.Type {
	case messagesv1.ChokeType: // receive choked from remote peer.
		p.Status.Remote = Choked
		return nil
	case messagesv1.UnChokeType: // receive unchoke from remote peer.
		p.Status.Remote = UnChoked
		return nil
	case messagesv1.InterestType: // recieve interest from remote peer.
		p.Interest.Remote = Interested
		return nil
	case messagesv1.NotInterestType: // receive notinterest from remote peer.
		p.Interest.Remote = NotInterested
		return nil
	case messagesv1.HaveType: // peer announce that he completed donwloading piecie with index.
		h := new(messagesv1.Have)
		if err := h.Deserialize(msg.Payload); err != nil {
			return fmt.Errorf("could not deserialize message %s: %w", msg.Type, err)
		}
		if err := p.bitfield.Set(h.Index); err != nil {
			return fmt.Errorf("could not set bitfield index %d: %w", h.Index, err)
		}
		return nil
	default:
		return fmt.Errorf("no implementation for processing message type: %s", msg.Type)
	}
}
