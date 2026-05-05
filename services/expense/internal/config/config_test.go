package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Success(t *testing.T) {
	t.Setenv("IMMUDB_ADDR", "localhost:3322")
	t.Setenv("IMMUDB_USERNAME", "testuser")
	t.Setenv("IMMUDB_PASSWORD", "testpass")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "localhost:3322", cfg.ImmudbAddr)
	assert.Equal(t, "testuser", cfg.ImmudbUsername)
	assert.Equal(t, "testpass", cfg.ImmudbPassword)
	assert.Equal(t, "8082", cfg.RESTPort)
	assert.Equal(t, "9082", cfg.GRPCPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
}

func TestLoad_RequiresImmudbAddr(t *testing.T) {
	// Ensure IMMUDB_ADDR is not set
	_ = os.Unsetenv("IMMUDB_ADDR")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "IMMUDB_ADDR is required")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("IMMUDB_ADDR", "localhost:3322")
	// Leave username/password unset to test defaults
	_ = os.Unsetenv("IMMUDB_USERNAME")
	_ = os.Unsetenv("IMMUDB_PASSWORD")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "immudb", cfg.ImmudbUsername)
	assert.Equal(t, "immudb", cfg.ImmudbPassword)
}

func TestLoad_CustomPorts(t *testing.T) {
	t.Setenv("IMMUDB_ADDR", "localhost:3322")
	t.Setenv("REST_PORT", "9999")
	t.Setenv("GRPC_PORT", "9998")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "9999", cfg.RESTPort)
	assert.Equal(t, "9998", cfg.GRPCPort)
}

func TestIsProduction(t *testing.T) {
	t.Setenv("IMMUDB_ADDR", "localhost:3322")

	t.Setenv("ENVIRONMENT", "development")
	cfg, _ := Load()
	assert.False(t, cfg.IsProduction())

	t.Setenv("ENVIRONMENT", "production")
	cfg, _ = Load()
	assert.True(t, cfg.IsProduction())
}
