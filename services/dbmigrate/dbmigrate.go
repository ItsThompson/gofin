// Package dbmigrate provides a shared database migration runner for gofin services.
// It wraps golang-migrate to apply filesystem-based SQL migrations at startup.
package dbmigrate

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Run connects to the database at dbURL, reads migrations from migrationsPath,
// and applies any pending migrations. It returns nil when all migrations are
// applied (including the case where no new migrations exist).
//
// A non-nil error is returned when:
//   - The migrations path doesn't exist or contains no migration files
//   - The database URL is invalid or unreachable
//   - A migration fails to apply (the database is left in a dirty state)
func Run(dbURL, migrationsPath string) error {
	if migrationsPath == "" {
		return fmt.Errorf("dbmigrate: migrations path is empty")
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("dbmigrate: creating migrator: %w", err)
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil {
			slog.Warn("dbmigrate: closing source", slog.String("error", sourceErr.Error()))
		}
		if dbErr != nil {
			slog.Warn("dbmigrate: closing database", slog.String("error", dbErr.Error()))
		}
	}()

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
