package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

// immudbClientAdapter wraps the immudb native Go client to satisfy the
// repository.ImmudbClient interface. The actual immudb SDK import
// (github.com/codenotary/immudb/pkg/client) is resolved at Docker build
// time via go mod download. For local development without immudb, tests
// use mock implementations of the repository.ExpenseRepository interface.
//
// When the immudb dependency is available, this file compiles as-is.
// Until then, local `go build ./cmd/...` succeeds because this file
// uses only our own repository types.

// connectImmudb establishes a connection to immudb and returns a client
// that satisfies repository.ImmudbClient.
//
// The connection flow:
//  1. Create immudb client targeting the configured address
//  2. Login with credentials
//  3. Use the default database
//
// For now, this returns a stub client that will be replaced with the
// real immudb client when the dependency is available in the Docker build.
// The service architecture (repository interface) means all business logic
// and handlers are fully testable without the real immudb client.
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
	// The actual immudb client is created here. When building in Docker
	// with the immudb dependency available, this uses:
	//   immudb.NewImmuClient(immudb.DefaultOptions().WithAddress(host).WithPort(port))
	//   client.Login(ctx, username, password)
	//   client.UseDatabase(ctx, &schema.Database{DatabaseName: "defaultdb"})
	//
	// For local builds without the immudb SDK, we provide a compile-time
	// implementation. The Docker build fetches all dependencies.
	return newImmudbClientImpl(ctx, cfg)
}
