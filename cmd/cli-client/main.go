package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/Despire/tinytorrent/p2p/client"
	"github.com/Despire/tinytorrent/torrent"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	if err := run(context.Background(), logger, os.Args[1:]); err != nil {
		logger.Error("stopping tinytorrent client due to encountered error while executing", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	if len(args) < 1 {
		return errors.New("no torrent file specified")
	}
	file, err := os.OpenFile(args[0], os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open torrent file %q: %w", args[0], err)
	}
	defer file.Close()

	t, err := torrent.From(file)
	if err != nil {
		return fmt.Errorf("failed to read torrent file %q: %w", args[0], err)
	}

	c, err := client.New(ctx, t, client.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("failed to initialize the client: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return c.Do(ctx)
}
