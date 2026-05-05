package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_RequiredVars(t *testing.T) {
	// Clear all env vars that Load reads
	_ = os.Unsetenv("FINANCE_DB_URL")
	_ = os.Unsetenv("EXPENSE_SERVICE_ADDR")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_DB_URL")
}

func TestLoad_MissingExpenseAddr(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	_ = os.Unsetenv("EXPENSE_SERVICE_ADDR")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EXPENSE_SERVICE_ADDR")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/test", cfg.DBUrl)
	assert.Equal(t, "localhost:9082", cfg.ExpenseServiceAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "8083", cfg.RESTPort)
	assert.Equal(t, "9083", cfg.GRPCPort)
	assert.False(t, cfg.IsProduction())
}

func TestLoad_Production(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("ENVIRONMENT", "production")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.IsProduction())
}
