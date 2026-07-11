package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

func TestLoad_RequiredVars(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing AUTH_DB_URL",
			env:     map[string]string{"JWT_SECRET": "secret"},
			wantErr: "AUTH_DB_URL is required",
		},
		{
			name:    "missing JWT_SECRET",
			env:     map[string]string{"AUTH_DB_URL": "postgres://localhost/test"},
			wantErr: "JWT_SECRET is required",
		},
		{
			name:    "both missing",
			env:     map[string]string{},
			wantErr: "AUTH_DB_URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setEnv(t, tt.env)

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_BcryptCostValidation(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "too low", value: "3", wantErr: "must be between 4 and 31"},
		{name: "too high", value: "32", wantErr: "must be between 4 and 31"},
		{name: "zero", value: "0", wantErr: "must be between 4 and 31"},
		{name: "negative", value: "-1", wantErr: "must be between 4 and 31"},
		{name: "not a number", value: "abc", wantErr: "must be an integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setEnv(t, map[string]string{
				"AUTH_DB_URL": "postgres://localhost/test",
				"JWT_SECRET":  "secret",
				"BCRYPT_COST": tt.value,
			})

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_BcryptCostValid(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantCost int
	}{
		{name: "minimum", value: "4", wantCost: 4},
		{name: "default when unset", value: "", wantCost: 12},
		{name: "maximum", value: "31", wantCost: 31},
		{name: "typical production", value: "12", wantCost: 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			env := map[string]string{
				"AUTH_DB_URL": "postgres://localhost/test",
				"JWT_SECRET":  "secret",
			}
			if tt.value != "" {
				env["BCRYPT_COST"] = tt.value
			}
			setEnv(t, env)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.wantCost, cfg.BcryptCost)
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{
		"AUTH_DB_URL": "postgres://localhost/test",
		"JWT_SECRET":  "secret",
	})

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "8081", cfg.RESTPort)
	assert.Equal(t, "9081", cfg.GRPCPort)
	assert.Equal(t, 12, cfg.BcryptCost)
}

func TestLoad_OverrideDefaults(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{
		"AUTH_DB_URL": "postgres://localhost/test",
		"JWT_SECRET":  "secret",
		"LOG_LEVEL":   "debug",
		"ENVIRONMENT": "production",
		"REST_PORT":   "9090",
		"GRPC_PORT":   "9091",
	})

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "9090", cfg.RESTPort)
	assert.Equal(t, "9091", cfg.GRPCPort)
}

func TestIsProduction(t *testing.T) {
	assert.False(t, (&Config{Environment: "development"}).IsProduction())
	assert.True(t, (&Config{Environment: "production"}).IsProduction())
	assert.True(t, (&Config{Environment: "staging"}).IsProduction())
}

func TestLoad_DurationDefaults(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{
		"AUTH_DB_URL": "postgres://localhost/test",
		"JWT_SECRET":  "secret",
	})

	cfg, err := Load()
	require.NoError(t, err)

	// Defaults are single-sourced from the JWTService's documented lifetimes.
	assert.Equal(t, service.DefaultAccessTokenTTL, cfg.JWTAccessTTL)
	assert.Equal(t, service.DefaultRefreshTokenTTL, cfg.JWTRefreshTTL)
	assert.Equal(t, 15*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 7*24*time.Hour, cfg.JWTRefreshTTL)
	assert.Equal(t, 5*time.Minute, cfg.CleanupInterval)
	assert.Equal(t, 30*time.Second, cfg.CleanupTimeout)
}

func TestLoad_DurationOverrides(t *testing.T) {
	clearEnv(t)
	setEnv(t, map[string]string{
		"AUTH_DB_URL":      "postgres://localhost/test",
		"JWT_SECRET":       "secret",
		"JWT_ACCESS_TTL":   "30m",
		"JWT_REFRESH_TTL":  "48h",
		"CLEANUP_INTERVAL": "1m",
		"CLEANUP_TIMEOUT":  "10s",
	})

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 48*time.Hour, cfg.JWTRefreshTTL)
	assert.Equal(t, time.Minute, cfg.CleanupInterval)
	assert.Equal(t, 10*time.Second, cfg.CleanupTimeout)
}

func TestLoad_InvalidDuration(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "access ttl", key: "JWT_ACCESS_TTL"},
		{name: "refresh ttl", key: "JWT_REFRESH_TTL"},
		{name: "cleanup interval", key: "CLEANUP_INTERVAL"},
		{name: "cleanup timeout", key: "CLEANUP_TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setEnv(t, map[string]string{
				"AUTH_DB_URL": "postgres://localhost/test",
				"JWT_SECRET":  "secret",
				tt.key:        "not-a-duration",
			})

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.key)
			assert.Contains(t, err.Error(), "must be a valid duration")
		})
	}
}

func TestRESTPort(t *testing.T) {
	clearEnv(t)
	assert.Equal(t, DefaultRESTPort, RESTPort())

	t.Setenv("REST_PORT", "9090")
	assert.Equal(t, "9090", RESTPort())
}

// clearEnv resets all config env vars to empty for test isolation. Load treats
// an empty value as unset (each read guards on ""), so the "missing var" cases
// behave as if the var were absent, while t.Setenv still restores the original
// on cleanup.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AUTH_DB_URL", "JWT_SECRET", "BCRYPT_COST",
		"LOG_LEVEL", "ENVIRONMENT", "REST_PORT", "GRPC_PORT",
		"COOKIE_DOMAIN", "JWT_ACCESS_TTL", "JWT_REFRESH_TTL",
		"CLEANUP_INTERVAL", "CLEANUP_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

// setEnv sets env vars for the duration of the test.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for key, val := range env {
		t.Setenv(key, val)
	}
}
