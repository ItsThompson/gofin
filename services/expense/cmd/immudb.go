package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

// connectImmudb returns the build-tagged ImmudbClient (in-memory stub for
// non-docker builds, real immudb client under the docker tag). The repository
// interface keeps business logic and handlers testable without a live immudb.
func connectImmudb(ctx context.Context, cfg *config.Config, logger *slog.Logger) (repository.ImmudbClient, error) {
	logger.Info("connecting to immudb",
		slog.String("addr", cfg.ImmudbAddr),
		slog.String("username", cfg.ImmudbUsername),
	)

	// Retry connection with backoff (immudb may not be ready immediately)
	var client repository.ImmudbClient
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		client, err = newImmudbClient(ctx, cfg)
		if err == nil {
			return client, nil
		}
		logger.Warn("immudb connection attempt failed, retrying...",
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to immudb after 10 attempts: %w", err)
}

// newImmudbClient creates a new immudb client connection.
// This is separated from connectImmudb to support retry logic.
func newImmudbClient(ctx context.Context, cfg *config.Config) (repository.ImmudbClient, error) {
	// newImmudbClientImpl is supplied by the build-tagged file (immudb_local.go or immudb_prod.go).
	return newImmudbClientImpl(ctx, cfg)
}
