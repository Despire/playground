package client

import (
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/status"
	"github.com/Despire/tinytorrent/p2p/messagesv1"
)

func (p *Client) handlePeer(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	closeConn := true

	defer p.wg.Done()
	defer func() {
		if closeConn {
			if err := conn.Close(); err != nil {
				p.logger.Debug("failed to closed leech connection", slog.String("leecher", addr))
			}
		}
	}()

	p.logger.Info("accepted new peer connection", slog.String("addr", addr))

	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		p.logger.Error("failed to set read deadline on new leecher connection, closing connection",
			slog.String("err", err.Error()),
			slog.String("leecher", addr),
		)
		return
	}

	var req [messagesv1.HandshakeLength]byte
	r, err := io.ReadFull(conn, req[:])
	if err != nil {
		p.logger.Error("failed to parse incoming peer message, closing connection",
			slog.String("err", err.Error()),
			slog.String("leecher", addr),
		)
		return
	}
	if r != len(req) {
		p.logger.Error("received invalid handshake message, closing connection",
			slog.String("leecher", addr),
		)
		return
	}

	var h messagesv1.Handshake
	if err := h.Deserialize(req[:]); err != nil {
		p.logger.Error("invalid handshake message received, closing connection",
			slog.String("err", err.Error()),
			slog.String("leecher", addr),
		)
		return
	}

	p.torrentsDownloading.Range(func(key, value any) bool {
		if key.(string) == h.InfoHash {
			if err := value.(*status.Tracker).AddLeecher(h.PeerID, conn); err != nil {
				p.logger.Error("failed to add new leecher",
					slog.String("leecher", addr),
					slog.String("err", err.Error()),
				)
				return false
			}
			p.logger.Info("successfully added leecher.", slog.String("leecher", addr))
			closeConn = false
			return false
		}
		return true
	})
}
