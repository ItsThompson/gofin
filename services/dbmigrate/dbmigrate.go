// Package dbmigrate provides a shared database migration runner for gofin services.
// It wraps golang-migrate to apply filesystem-based SQL migrations at startup.
package dbmigrate

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// Run connects to the database at dbURL, reads migrations from migrationsPath,
// and applies any pending migrations. It returns nil when all migrations are
// applied (including the case where no new migrations exist).
//
// If the connection URL contains a search_path parameter referencing a schema
// that doesn't yet exist, Run creates the schema before running migrations.
// This solves the chicken-and-egg problem where golang-migrate's postgres driver
// requires the schema to exist on connect.
//
// A non-nil error is returned when:
//   - The migrations path doesn't exist or contains no migration files
//   - The database URL is invalid or unreachable
//   - A migration fails to apply (the database is left in a dirty state)
func Run(dbURL, migrationsPath string) error {
	if migrationsPath == "" {
		return fmt.Errorf("dbmigrate: migrations path is empty")
	}

	if err := ensureSchema(dbURL); err != nil {
		return fmt.Errorf("dbmigrate: ensuring schema exists: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, dbURL)
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

// ensureSchema parses the search_path from the database URL and creates the
// schema if it doesn't exist. This must run before golang-migrate connects,
// because the postgres driver fails with "no schema" if search_path references
// a non-existent schema.
func ensureSchema(dbURL string) error {
	searchPath, connURL, err := rawConnectionURL(dbURL)
	if err != nil {
		return fmt.Errorf("parsing database URL: %w", err)
	}

	if searchPath == "" {
		return nil
	}

	db, err := sql.Open("postgres", connURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close() //nolint:errcheck

	stmt := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", searchPath)
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("creating schema %q: %w", searchPath, err)
	}

	slog.Info("dbmigrate: schema ensured", slog.String("schema", searchPath))
	return nil
}

// rawConnectionURL returns the search_path from dbURL along with a connection
// URL suitable for a plain lib/pq connection. It strips both the search_path
// (so the connection succeeds even if the schema doesn't exist yet) and any
// golang-migrate driver params (the x-* family, e.g. x-migrations-table). Those
// x-* params are consumed by golang-migrate's own connection, not by a raw
// lib/pq connection, which would otherwise reject them as unknown server
// configuration parameters.
func rawConnectionURL(dbURL string) (searchPath, connURL string, err error) {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return "", "", err
	}

	q := parsed.Query()
	searchPath = q.Get("search_path")
	q.Del("search_path")
	for key := range q {
		if strings.HasPrefix(key, "x-") {
			q.Del(key)
		}
	}
	parsed.RawQuery = q.Encode()

	return searchPath, parsed.String(), nil
}
