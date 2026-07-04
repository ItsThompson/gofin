package dbmigrate

import "testing"

func TestRawConnectionURL(t *testing.T) {
	tests := []struct {
		name           string
		dbURL          string
		wantSearchPath string
		wantConnURL    string
	}{
		{
			name:           "no search_path leaves url unchanged",
			dbURL:          "postgres://u:p@localhost:5432/db?sslmode=disable",
			wantSearchPath: "",
			wantConnURL:    "postgres://u:p@localhost:5432/db?sslmode=disable",
		},
		{
			name:           "search_path is extracted and removed",
			dbURL:          "postgres://u:p@localhost:5432/db?search_path=finance&sslmode=disable",
			wantSearchPath: "finance",
			wantConnURL:    "postgres://u:p@localhost:5432/db?sslmode=disable",
		},
		{
			name:           "golang-migrate x- params are stripped for the raw connection",
			dbURL:          "postgres://u:p@localhost:5432/db?search_path=auth&x-migrations-table=schema_migrations_auth&sslmode=disable",
			wantSearchPath: "auth",
			wantConnURL:    "postgres://u:p@localhost:5432/db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchPath, connURL, err := rawConnectionURL(tt.dbURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if searchPath != tt.wantSearchPath {
				t.Errorf("searchPath: got %q, want %q", searchPath, tt.wantSearchPath)
			}
			if connURL != tt.wantConnURL {
				t.Errorf("connURL: got %q, want %q", connURL, tt.wantConnURL)
			}
		})
	}
}

func TestRawConnectionURL_InvalidURL(t *testing.T) {
	_, _, err := rawConnectionURL("://not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}
