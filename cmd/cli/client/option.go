package client

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/Despire/tinytorrent/cmd/cli/client/internal/build"
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

func WithAction(action Action) Option {
	return func(client *Client) {
		client.action = action
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

	c.port = 6882 // default port this client will listen on.

	c.action = Leech

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
