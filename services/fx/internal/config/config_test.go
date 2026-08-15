package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ReadsFxEnvironment(t *testing.T) {
	t.Setenv("OPEN_EXCHANGE_RATES_APP_ID", "app-id")
	t.Setenv("OPEN_EXCHANGE_RATES_BASE_URL", "http://provider.test/latest.json")
	t.Setenv("FX_PROVIDER_TIMEOUT", "1500ms")
	t.Setenv("FX_PROVIDER_RETRY_COUNT", "3")
	t.Setenv("FX_CACHE_MAX_AGE", "30m")
	t.Setenv("REST_PORT", "18085")
	t.Setenv("GRPC_PORT", "19085")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ENVIRONMENT", "production")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "app-id", cfg.OpenExchangeRatesAppID)
	assert.Equal(t, "http://provider.test/latest.json", cfg.ProviderBaseURL)
	assert.Equal(t, 1500*time.Millisecond, cfg.ProviderTimeout)
	assert.Equal(t, 3, cfg.ProviderRetryCount)
	assert.Equal(t, 30*time.Minute, cfg.CacheMaxAge)
	assert.Equal(t, "18085", cfg.RESTPort)
	assert.Equal(t, "19085", cfg.GRPCPort)
	assert.True(t, cfg.IsProduction())
}

func TestLoad_AllowsEmptyProviderAppID(t *testing.T) {
	cfg, err := Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.OpenExchangeRatesAppID)
}

func TestLoad_RejectsInvalidDuration(t *testing.T) {
	t.Setenv("OPEN_EXCHANGE_RATES_APP_ID", "app-id")
	t.Setenv("FX_CACHE_MAX_AGE", "0s")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FX_CACHE_MAX_AGE")
}
