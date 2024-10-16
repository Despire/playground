package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/Despire/tinytorrent/torrent"
)

// Peer represents a single instance of a peer within
// the BitTorrent network.
type Peer struct {
	id     string
	logger *slog.Logger
	port   int
}

func New(ctx context.Context, torrent *torrent.MetaInfoFile, opts ...Option) (*Peer, error) {
	c := &Peer{}
	defaults(c)

	for _, o := range opts {
		o(c)
	}

	c.logger = c.logger.With(slog.Group(c.id, slog.String("id", c.id)))

	params := url.Values{
		"info_hash": {string(torrent.Metadata.Hash[:])},
		"peer_id":   {c.id},
		"port":      {fmt.Sprint(c.port)},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s?%s", torrent.Announce, params.Encode()),
		nil,
	)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("initiating communication with tracker", "url", req.URL.String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	fmt.Println(resp.Status)
	fmt.Println(resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// TODO: implement me.
	panic(string(body))

	return c, nil
}

func (c *Peer) Do(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("received signal to stop")
			return nil
		}
	}
}
