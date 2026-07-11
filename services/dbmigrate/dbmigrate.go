// Package dbmigrate provides a shared database migration runner for gofin services.
// It wraps golang-migrate to apply filesystem-based SQL migrations at startup.
package dbmigrate

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/lib/pq"
)

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
