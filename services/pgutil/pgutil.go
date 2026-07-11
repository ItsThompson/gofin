// Package pgutil holds the small pgx/pgtype boilerplate helpers that every
// Postgres-backed GoFin service would otherwise re-implement: parsing a string
// into a pgtype.UUID, detecting pgx's no-rows sentinel, and detecting a
// Postgres unique-constraint violation.
package pgutil

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// sqlStateUniqueViolation is the Postgres SQLSTATE for a unique-constraint
// violation (class 23 integrity-constraint violation).
const sqlStateUniqueViolation = "23505"

// ParseUUID parses s into a pgtype.UUID. On failure it returns the zero value
// and a wrapped "parsing UUID: ..." error so every call site reports a
// consistent message.
func ParseUUID(s string) (pgtype.UUID, error) {
	var uid pgtype.UUID
	if err := uid.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parsing UUID: %w", err)
	}
	return uid, nil
}

// IsNoRows reports whether err is (or wraps) pgx.ErrNoRows.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation reports whether err is (or wraps) a Postgres
// unique-constraint violation (SQLSTATE 23505). When ok is true, constraint is
// the name of the violated constraint (empty if the server did not report one).
func IsUniqueViolation(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlStateUniqueViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}
