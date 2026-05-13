package dbmigrate_test

import (
	"embed"
	"os"
	"testing"

	"github.com/ItsThompson/gofin/services/dbmigrate"
)

//go:embed testdata/valid_migrations/*.sql
var validMigrationsFS embed.FS

//go:embed testdata/invalid_migrations/*.sql
var invalidMigrationsFS embed.FS

func TestRunWithFS_EmptySubdir(t *testing.T) {
	var fs embed.FS
	err := dbmigrate.RunWithFS("postgres://user:pass@localhost/db", fs, "")
	if err == nil {
		t.Fatal("expected error for empty subdir, got nil")
	}
	expected := "dbmigrate: subdir is empty"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestRunWithFS_InvalidSubdir(t *testing.T) {
	var fs embed.FS
	err := dbmigrate.RunWithFS("postgres://user:pass@localhost/db", fs, "nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent subdir in FS, got nil")
	}
}

func TestRunWithFS_InvalidDBURL(t *testing.T) {
	err := dbmigrate.RunWithFS(
		"postgres://invalid:invalid@localhost:59999/nonexistent?connect_timeout=1",
		validMigrationsFS,
		"testdata/valid_migrations",
	)
	if err == nil {
		t.Fatal("expected error for unreachable database, got nil")
	}
}

// TestRunWithFS_Success requires a running PostgreSQL instance.
// Set TEST_DATABASE_URL to run this test (e.g., via `just dev-infra`).
func TestRunWithFS_Success(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping integration test")
	}

	// First run: applies migrations
	err := dbmigrate.RunWithFS(dbURL, validMigrationsFS, "testdata/valid_migrations")
	if err != nil {
		t.Fatalf("expected nil on first run, got: %v", err)
	}

	// Second run: idempotent (no new migrations)
	err = dbmigrate.RunWithFS(dbURL, validMigrationsFS, "testdata/valid_migrations")
	if err != nil {
		t.Fatalf("expected nil on idempotent rerun, got: %v", err)
	}
}

// TestRunWithFS_InvalidSQL requires a running PostgreSQL instance.
func TestRunWithFS_InvalidSQL(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping integration test")
	}

	err := dbmigrate.RunWithFS(dbURL, invalidMigrationsFS, "testdata/invalid_migrations")
	if err == nil {
		t.Fatal("expected error for invalid SQL migration, got nil")
	}
}
