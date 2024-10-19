package client

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"

	"github.com/Despire/tinytorrent/p2p/client/internal/build"
)

type Option func(client *Client)

func WithPort(port int) Option {
	return func(client *Client) {
		client.port = port
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		client.logger = logger
	}
}

func defaults(c *Client) {
	info := build.Information()

	id := sha512.Sum512([]byte(info.ClientID + info.ClientVersion))
	c.id = fmt.Sprintf("%s%s%s", info.ClientID, info.ClientVersion, hex.EncodeToString(id[:])[:14])

	c.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	c.port = 6881 + rand.IntN(9)

	c.logger.Debug("Build Information",
		slog.String("ClientID", info.ClientID),
		slog.String("ClientVersion", info.ClientVersion),
		slog.String("BuildDate", info.BuildDate.String()),
		slog.String("BuildHash", info.BuildHash),
		slog.String("GoVersion", info.GoVersion),
		slog.String("Compiler", info.Compiler),
		slog.String("Platform", info.Platform),
		slog.String("Architecture", info.Architecture),
	)
}
