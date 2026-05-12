package dbmigrate

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunWithFS applies database migrations using an embedded filesystem.
// The fsys parameter should be an embed.FS containing the migration files,
// and subdir is the path within that FS where migration files live
// (e.g., "db/migrations").
//
// This eliminates the need to COPY migration files into the container at runtime,
// making binaries fully self-contained for distroless/scratch images.
func RunWithFS(dbURL string, fsys fs.FS, subdir string) error {
	if subdir == "" {
		return fmt.Errorf("dbmigrate: subdir is empty")
	}

	if err := ensureSchema(dbURL); err != nil {
		return fmt.Errorf("dbmigrate: ensuring schema exists: %w", err)
	}

	source, err := iofs.New(fsys, subdir)
	if err != nil {
		return fmt.Errorf("dbmigrate: creating iofs source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		return fmt.Errorf("dbmigrate: creating migrator: %w", err)
	}
	defer m.Close() //nolint:errcheck

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("dbmigrate: no new migrations to apply")
		return nil
	}
	if err != nil {
		return fmt.Errorf("dbmigrate: applying migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("dbmigrate: migrations applied successfully",
		slog.Uint64("version", uint64(version)),
		slog.Bool("dirty", dirty),
	)

	return nil
}
