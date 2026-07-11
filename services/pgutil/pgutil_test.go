package pgutil_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ItsThompson/gofin/services/pgutil"
)

func TestParseUUID_Valid(t *testing.T) {
	want := uuid.New()

	got, err := pgutil.ParseUUID(want.String())
	if err != nil {
		t.Fatalf("ParseUUID(%q) returned error: %v", want, err)
	}
	if !got.Valid {
		t.Errorf("ParseUUID(%q).Valid = false, want true", want)
	}
	// The parsed bytes must round-trip back to the original UUID. This is the
	// exact conversion the call sites use (uuid.UUID(b).String()).
	if roundTrip := uuid.UUID(got.Bytes); roundTrip != want {
		t.Errorf("ParseUUID(%q) round-trip = %v, want %v", want, roundTrip, want)
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	got, err := pgutil.ParseUUID("not-a-uuid")
	if err == nil {
		t.Fatal("ParseUUID(\"not-a-uuid\") returned nil error, want error")
	}
	if got.Valid {
		t.Errorf("ParseUUID(\"not-a-uuid\").Valid = true, want false")
	}
	if !strings.HasPrefix(err.Error(), "parsing UUID: ") {
		t.Errorf("error = %q, want %q prefix", err.Error(), "parsing UUID: ")
	}
	// The wrap must be verb-style (errors.Unwrap non-nil) so callers can
	// errors.Is/As the underlying scan cause.
	if errors.Unwrap(err) == nil {
		t.Error("ParseUUID error does not wrap the underlying cause")
	}
}

func TestIsNoRows(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "bare ErrNoRows", err: pgx.ErrNoRows, want: true},
		{name: "wrapped ErrNoRows", err: fmt.Errorf("query row: %w", pgx.ErrNoRows), want: true},
		{name: "unrelated error", err: errors.New("connection reset"), want: false},
		{name: "nil error", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pgutil.IsNoRows(tt.err); got != tt.want {
				t.Errorf("IsNoRows(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantConstraint string
		wantOK         bool
	}{
		{
			name:           "unique violation",
			err:            &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"},
			wantConstraint: "users_email_key",
			wantOK:         true,
		},
		{
			name:           "wrapped unique violation",
			err:            fmt.Errorf("insert user: %w", &pgconn.PgError{Code: "23505", ConstraintName: "tags_user_id_name_key"}),
			wantConstraint: "tags_user_id_name_key",
			wantOK:         true,
		},
		{
			name:   "different sqlstate (foreign key violation)",
			err:    &pgconn.PgError{Code: "23503", ConstraintName: "fk_user"},
			wantOK: false,
		},
		{
			name:   "non-pg error",
			err:    errors.New("connection reset"),
			wantOK: false,
		},
		{
			name:   "nil error",
			err:    nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConstraint, gotOK := pgutil.IsUniqueViolation(tt.err)
			if gotOK != tt.wantOK {
				t.Errorf("IsUniqueViolation(%v) ok = %v, want %v", tt.err, gotOK, tt.wantOK)
			}
			if gotConstraint != tt.wantConstraint {
				t.Errorf("IsUniqueViolation(%v) constraint = %q, want %q", tt.err, gotConstraint, tt.wantConstraint)
			}
		})
	}
}
