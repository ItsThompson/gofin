package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearEnv blanks every datarights env var so each test starts from a known
// baseline. Load treats an empty value as unset (falling back to the default),
// except DATARIGHTS_DB_URL which is required.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATARIGHTS_DB_URL", "LOG_LEVEL", "ENVIRONMENT", "REST_PORT",
		"AUTH_SERVICE_ADDR", "EXPENSE_SERVICE_ADDR", "FINANCE_SERVICE_ADDR",
		"EXPORT_MAX_CONCURRENT", "EXPORT_TIMEOUT_SECONDS", "DELETION_TIMEOUT_SECONDS",
		"EMAIL_ENABLED", "RESEND_API_KEY", "EMAIL_FROM", "BRAND_TOKENS_PATH",
		"PROTECTED_USERNAMES",
	} {
		t.Setenv(key, "")
	}
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for key, val := range env {
		t.Setenv(key, val)
	}
}

func TestLoad_MissingDBURL(t *testing.T) {
	clearEnv(t)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATARIGHTS_DB_URL is required")
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{"DATARIGHTS_DB_URL": "postgres://localhost/test"})

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, DefaultRESTPort, cfg.RESTPort)
	assert.Equal(t, "auth-service:9081", cfg.AuthServiceAddr)
	assert.Equal(t, "expense-service:9082", cfg.ExpenseServiceAddr)
	assert.Equal(t, "finance-service:9083", cfg.FinanceServiceAddr)
	assert.Equal(t, 5, cfg.MaxConcurrent)
	assert.Equal(t, 5*time.Minute, cfg.ExportTimeout)
	assert.Equal(t, 5*time.Minute, cfg.DeletionTimeout)
	assert.True(t, cfg.EmailEnabled)
	assert.Equal(t, "gofin <noreply@usegofin.com>", cfg.EmailFrom)
	assert.Equal(t, "/app/tokens/brand.json", cfg.BrandTokensPath)
	assert.Equal(t, []string{"admin", "thompson"}, cfg.ProtectedUsernames)
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{
		"DATARIGHTS_DB_URL":        "postgres://localhost/test",
		"LOG_LEVEL":                "debug",
		"ENVIRONMENT":              "production",
		"REST_PORT":                "1234",
		"EXPORT_MAX_CONCURRENT":    "9",
		"EXPORT_TIMEOUT_SECONDS":   "30",
		"DELETION_TIMEOUT_SECONDS": "45",
		"EMAIL_ENABLED":            "false",
	})

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "production", cfg.Environment)
	assert.True(t, cfg.IsProduction())
	assert.Equal(t, "1234", cfg.RESTPort)
	assert.Equal(t, 9, cfg.MaxConcurrent)
	assert.Equal(t, 30*time.Second, cfg.ExportTimeout)
	assert.Equal(t, 45*time.Second, cfg.DeletionTimeout)
	assert.False(t, cfg.EmailEnabled)
}

func TestRESTPort(t *testing.T) {
	clearEnv(t)
	assert.Equal(t, DefaultRESTPort, RESTPort())

	setEnv(t, map[string]string{"REST_PORT": "9999"})
	assert.Equal(t, "9999", RESTPort())
}

func TestLoad_ProtectedUsernames(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  []string
	}{
		{name: "unset uses default", set: false, want: []string{"admin", "thompson"}},
		{name: "custom list", value: "root,superuser", set: true, want: []string{"root", "superuser"}},
		{name: "trims spaces and drops empties", value: " a , b ,, c ", set: true, want: []string{"a", "b", "c"}},
		{name: "whitespace only uses default", value: "   ", set: true, want: []string{"admin", "thompson"}},
		{name: "commas only uses default", value: ",,", set: true, want: []string{"admin", "thompson"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			env := map[string]string{"DATARIGHTS_DB_URL": "postgres://localhost/test"}
			if tt.set {
				env["PROTECTED_USERNAMES"] = tt.value
			}
			setEnv(t, env)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.ProtectedUsernames)
		})
	}
}

func TestLoad_ProtectedUsernamesDefaultNotAliased(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{"DATARIGHTS_DB_URL": "postgres://localhost/test"})

	cfg, err := Load()
	require.NoError(t, err)

	// Mutating the loaded slice must not corrupt the shared package default.
	cfg.ProtectedUsernames[0] = "mutated"
	assert.Equal(t, "admin", DefaultProtectedUsernames[0])
}
