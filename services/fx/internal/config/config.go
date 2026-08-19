package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	DefaultRESTPort           = "8085"
	DefaultProviderBaseURL    = "https://openexchangerates.org/api/latest.json"
	defaultGRPCPort           = "9085"
	defaultProviderTimeout    = 2 * time.Second
	defaultProviderRetryCount = 2
	defaultCacheMaxAge        = time.Hour
)

type Config struct {
	OpenExchangeRatesAppID string
	ProviderBaseURL        string
	ProviderTimeout        time.Duration
	ProviderRetryCount     int
	CacheMaxAge            time.Duration
	LogLevel               string
	Environment            string
	RESTPort               string
	GRPCPort               string
}

func ResolveRESTPort() string {
	if port := os.Getenv("REST_PORT"); port != "" {
		return port
	}
	return DefaultRESTPort
}

func Load() (*Config, error) {
	appID := os.Getenv("OPEN_EXCHANGE_RATES_APP_ID")

	providerTimeout, err := durationFromEnv("FX_PROVIDER_TIMEOUT", defaultProviderTimeout)
	if err != nil {
		return nil, err
	}
	retryCount, err := intFromEnv("FX_PROVIDER_RETRY_COUNT", defaultProviderRetryCount)
	if err != nil {
		return nil, err
	}
	cacheMaxAge, err := durationFromEnv("FX_CACHE_MAX_AGE", defaultCacheMaxAge)
	if err != nil {
		return nil, err
	}

	return &Config{
		OpenExchangeRatesAppID: appID,
		ProviderBaseURL:        stringFromEnv("OPEN_EXCHANGE_RATES_BASE_URL", DefaultProviderBaseURL),
		ProviderTimeout:        providerTimeout,
		ProviderRetryCount:     retryCount,
		CacheMaxAge:            cacheMaxAge,
		LogLevel:               stringFromEnv("LOG_LEVEL", "info"),
		Environment:            stringFromEnv("ENVIRONMENT", "development"),
		RESTPort:               ResolveRESTPort(),
		GRPCPort:               stringFromEnv("GRPC_PORT", defaultGRPCPort),
	}, nil
}

func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}

func stringFromEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", name, raw, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", name, raw)
	}
	return value, nil
}

func intFromEnv(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", name, raw, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be zero or greater, got %q", name, raw)
	}
	return value, nil
}
