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
	opts := &slog.HandlerOptions{
		AddSource: true,
	}

	switch e := os.Getenv("TINY_LOG_LEVEL"); e {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		opts.Level = slog.LevelInfo

	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	if err := run(context.Background(), logger, os.Args[1:]); err != nil {
		logger.Error("stopping tinytorrent client due to encountered error while executing", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	if len(args) < 1 {
		return errors.New("no torrent file specified")
	}
	action := "leech"
	if len(args) == 2 {
		switch args[1] {
		case "leech":
		case "both":
			action = "both"
		default:
			return fmt.Errorf("unsupported torrent action %v, supported only (leech|seed|both)", args[1])
		}
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

	c, err := client.New(client.WithLogger(logger), client.WithAction(client.Action(action)))
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
	for {
		select {
		case <-ctx.Done():
			logger.Warn("interrupt signal received")
			return c.Close()
		case err, _ := <-done:
			if err != nil {
				if err := c.Close(); err != nil {
					logger.Error("failed to close client", "error", err)
				}
				return fmt.Errorf("failed to wait for work on torrent %s to finish: %w", id, err)
			}
		}
	}
}
