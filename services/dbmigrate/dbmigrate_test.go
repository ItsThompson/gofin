package dbmigrate_test

import (
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

func TestRun_InvalidPath(t *testing.T) {
	err := dbmigrate.Run(
		"postgres://user:pass@localhost/db",
		"/nonexistent/path/to/migrations",
	)
	if err == nil {
		t.Fatal("expected error for nonexistent migrations path, got nil")
	}
}

func TestRun_InvalidDBURL(t *testing.T) {
	// Use a valid path format but invalid DB URL to test URL parsing.
	// The file source will fail first if the path doesn't exist, so we test
	// with a path that won't have migration files but might resolve.
	err := dbmigrate.Run(
		"not-a-valid-url",
		"./testdata/migrations",
	)
	if err == nil {
		t.Fatal("expected error for invalid database URL, got nil")
	}
}
