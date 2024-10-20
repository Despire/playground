package status

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/tracker"
	"github.com/Despire/tinytorrent/p2p/peer"
	"github.com/Despire/tinytorrent/torrent"
)

// Tracker wraps all necessary information for tracking
// the status of a torrent file
type Tracker struct {
	Torrent    *torrent.MetaInfoFile
	Uploaded   int64
	Downloaded int64
	Peers      map[string]*peer.Peer
}

func (t *Tracker) UpdatePeers(clientID string, logger *slog.Logger, resp *tracker.Response) error {
	var errAll error

	for _, r := range resp.Peers {
		ip := net.JoinHostPort(r.IP, fmt.Sprint(r.Port))
		logger.Info("initiating connection to",
			slog.String("peer_ip", ip),
			slog.String("url", t.Torrent.Announce),
			slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
		)
		p, ok := t.Peers[ip]
		if !ok {
			np, err := peer.New(logger, r.PeerID, ip)
			if err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to connect to peer %s: %w", ip, err))
				continue
			}

			t.Peers[ip] = np
			p = np
		}

		switch p.ConnectionStatus {
		case peer.ConnectionPending:
			logger.Info("initiating handshake",
				slog.String("peer_ip", ip),
				slog.String("peer_id", p.Id),
				slog.String("url", t.Torrent.Announce),
				slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
			)
			if err := p.HandshakeV1(string(t.Torrent.Metadata.Hash[:]), clientID); err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to handshake with peer %s: %w", ip, err))
			}
		case peer.ConnectionKilled:
			logger.Info("deleting peer from tracker",
				slog.String("peer_ip", ip),
				slog.String("peer_id", p.Id),
				slog.String("url", t.Torrent.Announce),
				slog.String("info_hash", string(t.Torrent.Metadata.Hash[:])),
			)
			delete(t.Peers, ip)
		}
	}

	return errAll
}

func (t *Tracker) Stop() error {
	var errAll error

	for ip, p := range t.Peers {
		if err := p.Close(); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("peer %q failed to close: %w", ip, err))
		}
	}

	return errAll
}
