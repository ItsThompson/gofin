package serverkit

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/dbmigrate"
)

// ConnectPostgres runs embedded migrations, opens a pgx pool, and pings it,
// collapsing the three-step dbmigrate.RunWithFS + pgxpool.New + pool.Ping dance
// every Postgres-backed service repeats. The migrations FS is expected to hold
// the SQL files at its root (the ".subdir" convention used by every service).
//
// On success the caller owns pool.Close() (via defer). On any failure the pool
// is closed before returning, so a failed connect never leaks a pool.
func ConnectPostgres(ctx context.Context, dbURL string, migrations fs.FS) (*pgxpool.Pool, error) {
	if err := dbmigrate.RunWithFS(dbURL, migrations, "."); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}
