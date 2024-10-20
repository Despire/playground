package peer

import (
	"errors"
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
		if err := p.Conn.SetReadDeadline(time.Now().Add(KeepAliveTimeout)); err != nil {
			p.logger.Info("failed to set read deadline to KeepAliveTimeout",
				slog.String("err", err.Error()),
			)
			continue
		}

		msg, err := messagesv1.Identify(p.Conn)
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
	}

	p.logger.Info("peer connection shutting down")

	if err := p.Close(); err != nil {
		p.logger.Info("failed to close connection", slog.String("err", err.Error()))
	}
}
