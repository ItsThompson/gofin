package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// clearEnv unsets all config env vars so tests are isolated.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AUTH_DB_URL", "JWT_SECRET", "BCRYPT_COST",
		"LOG_LEVEL", "ENVIRONMENT", "REST_PORT", "GRPC_PORT",
	} {
		t.Setenv(key, "")
		// t.Setenv restores original value on cleanup, but we need
		// the var to be truly unset for "missing" tests
	}
}

// setEnv sets env vars for the duration of the test.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for key, val := range env {
		t.Setenv(key, val)
	}
}
