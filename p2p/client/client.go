package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Despire/tinytorrent/bencoding"
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

	// TODO: rewrite this with new api in tracker.go
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request send to tracker at %s returned status code: %v, body: %s", req.URL.String(), resp.StatusCode, body)
	}

	v, err := bencoding.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if typ := v.Type(); typ != bencoding.DictionaryType {
		return nil, fmt.Errorf("expected response from tracker at %s to be a bencoded dictionary, got %s", req.URL.String(), typ)
	}

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
