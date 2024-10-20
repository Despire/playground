package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/Despire/tinytorrent/cmd/cli/client"
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

	c, err := client.New(client.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("failed to initialize the client: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	id, err := c.WorkOn(t)
	if err != nil {
		return fmt.Errorf("failed to start work on: %w", err)
	}

	done := c.WaitFor(id)
	select {
	case <-ctx.Done():
		logger.Warn("interrupt signal received")
		if err := c.Close(); err != nil {
			logger.Error("failed to close client", "error", err)
		}
		logger.Info("waiting for torrent to finish", slog.String("torrent", id))
		if err := <-done; err != nil {
			return fmt.Errorf("failed to wait for work on torrent %s to finish: %w", id, err)
		}
		return nil
	case err := <-done:
		if err := c.Close(); err != nil {
			logger.Error("failed to close client", "error", err)
		}
		if err != nil {
			return fmt.Errorf("failed to wait for work on torrent %s to finish: %w", id, err)
		}
		return nil
	}
}
