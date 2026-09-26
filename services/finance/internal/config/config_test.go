package config

import (
	"os"
	"testing"
	"time"

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
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EXPENSE_SERVICE_ADDR")
}

func TestLoad_MissingFxAddr(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	_ = os.Unsetenv("FX_SERVICE_ADDR")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FX_SERVICE_ADDR")
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/test", cfg.DBUrl)
	assert.Equal(t, "localhost:9082", cfg.ExpenseServiceAddr)
	assert.Equal(t, "localhost:9085", cfg.FxServiceAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "8083", cfg.RESTPort)
	assert.Equal(t, "9083", cfg.GRPCPort)
	assert.False(t, cfg.ResultCacheEnabled)
	assert.Equal(t, 256, cfg.ResultCacheMaxEntries)
	assert.Equal(t, int64(64*1024*1024), cfg.ResultCacheMaxBytes)
	assert.Equal(t, 48*time.Hour, cfg.ResultCacheMaxAge)
	assert.Equal(t, 2*time.Minute, cfg.ResultCacheLease)
	assert.Equal(t, 5*time.Second, cfg.ResultCacheTimeout)
	assert.Less(t, cfg.ResultCacheTimeout, cfg.ResultCacheLease)
	assert.False(t, cfg.IsProduction())
}

func TestLoad_CustomResultCacheConfig(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_ENABLED", "false")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_ENTRIES", "9")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_BYTES", "4096")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_ENTRY_BYTES", "1024")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_AGE", "3h")
	t.Setenv("FINANCE_RESULT_CACHE_VALIDATION_LEASE", "30s")
	t.Setenv("FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT", "2s")

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.ResultCacheEnabled)
	assert.Equal(t, 9, cfg.ResultCacheMaxEntries)
	assert.Equal(t, int64(4096), cfg.ResultCacheMaxBytes)
	assert.Equal(t, int64(1024), cfg.ResultCacheMaxEntryBytes)
	assert.Equal(t, 3*time.Hour, cfg.ResultCacheMaxAge)
	assert.Equal(t, 30*time.Second, cfg.ResultCacheLease)
	assert.Equal(t, 2*time.Second, cfg.ResultCacheTimeout)
}

func TestLoad_RejectsResultCacheEntriesBelowStoreCount(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_ENTRIES", "8")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_MAX_ENTRIES must be at least")
}

func TestLoad_RejectsResultCacheBytesBelowStoreCount(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_BYTES", "8")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_MAX_BYTES must be at least")
}

func TestLoad_RejectsResultCacheAgeAboveUpperBound(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_AGE", "48h1m")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_MAX_AGE must be at most")
}

func TestLoad_RejectsResultCacheBudgetAboveProcessSafeLimit(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_MAX_BYTES", "268435457")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_MAX_BYTES must be at most")
}

func TestLoad_RejectsLeaseAtOrBelowExpenseCallbackBound(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_VALIDATION_LEASE", "1s")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_VALIDATION_LEASE must be greater than")
}

func TestLoad_RejectsValidationTimeoutAtOrAboveLease(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("FINANCE_RESULT_CACHE_VALIDATION_LEASE", "5s")
	t.Setenv("FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT", "5s")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT must be less than")
}

func TestLoad_Production(t *testing.T) {
	t.Setenv("FINANCE_DB_URL", "postgres://localhost/test")
	t.Setenv("EXPENSE_SERVICE_ADDR", "localhost:9082")
	t.Setenv("FX_SERVICE_ADDR", "localhost:9085")
	t.Setenv("ENVIRONMENT", "production")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.IsProduction())
}
