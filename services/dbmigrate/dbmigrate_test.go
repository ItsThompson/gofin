package dbmigrate_test

import (
	"os"
	"testing"

	"github.com/ItsThompson/gofin/services/dbmigrate"
)

func TestRun_EmptyPath(t *testing.T) {
	err := dbmigrate.Run("postgres://user:pass@localhost/db", "")
	if err == nil {
		t.Fatal("expected error for empty migrations path, got nil")
	}
	expected := "dbmigrate: migrations path is empty"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestRun_NonexistentPath(t *testing.T) {
	err := dbmigrate.Run(
		"postgres://user:pass@localhost/db",
		"/nonexistent/path/to/migrations",
	)
	if err == nil {
		t.Fatal("expected error for nonexistent migrations path, got nil")
	}
}

func TestRun_InvalidDBURL(t *testing.T) {
	// Use valid migration files but an invalid DB URL to test connection failure.
	err := dbmigrate.Run(
		"postgres://invalid:invalid@localhost:59999/nonexistent?connect_timeout=1",
		"./testdata/valid_migrations",
	)
	if err == nil {
		t.Fatal("expected error for unreachable database, got nil")
	}
}

// TestRun_Success requires a running PostgreSQL instance.
// Set TEST_DATABASE_URL to run this test (e.g., via `just dev-infra`).
func TestRun_Success(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping integration test")
	}

	// First run: applies migrations
	err := dbmigrate.Run(dbURL, "./testdata/valid_migrations")
	if err != nil {
		t.Fatalf("expected nil on first run, got: %v", err)
	}

	// Second run: idempotent (no new migrations)
	err = dbmigrate.Run(dbURL, "./testdata/valid_migrations")
	if err != nil {
		t.Fatalf("expected nil on idempotent rerun, got: %v", err)
	}
}

// TestRun_InvalidSQL requires a running PostgreSQL instance.
// Verifies that a malformed migration returns an error.
func TestRun_InvalidSQL(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping integration test")
	}

	err := dbmigrate.Run(dbURL, "./testdata/invalid_migrations")
	if err == nil {
		t.Fatal("expected error for invalid SQL migration, got nil")
	}
}
