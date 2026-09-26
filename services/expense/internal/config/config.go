package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	financeconfig "github.com/ItsThompson/gofin/services/finance/config"
)

// DefaultRESTPort is the single source of truth for the expense REST port
// default. It backs both the listener (via Load) and the --healthcheck probe
// (via ResolveRESTPort) so the two never desync.
const DefaultRESTPort = "8082"

// ReportDomain tags every error report this service makes; it is a query
// dimension shared across services so cross-project Sentry queries work. The
// closed set of domains is documented in docs/error-handling.md.
const ReportDomain = "expenses"

const (
	defaultReadCacheEnabled       = false
	defaultReadCacheMaxEntries    = 256
	defaultReadCacheMaxBytes      = 64 * 1024 * 1024
	defaultReadCacheMaxEntryBytes = 16 * 1024 * 1024
	defaultReadCacheMaxAge        = 48 * time.Hour
	defaultFinanceEvictionTimeout = financeconfig.MaxExpenseFinanceEvictionTimeout
)

// ResolveRESTPort returns the REST port from REST_PORT, falling back to
// DefaultRESTPort. The --healthcheck branch runs before Load, so it calls this
// to probe the same port the listener will bind.
func ResolveRESTPort() string {
	if p := os.Getenv("REST_PORT"); p != "" {
		return p
	}
	return DefaultRESTPort
}

// Config holds all configuration for the expense service, loaded from environment variables.
type Config struct {
	ImmudbAddr             string
	ImmudbUsername         string
	ImmudbPassword         string
	FinanceServiceAddr     string
	FxServiceAddr          string
	LogLevel               string
	Environment            string
	RESTPort               string
	GRPCPort               string
	ReadCacheEnabled       bool
	ReadCacheMaxEntries    int
	ReadCacheMaxBytes      int64
	ReadCacheMaxEntryBytes int64
	ReadCacheMaxAge        time.Duration
	FinanceEvictionTimeout time.Duration
}

// Load reads configuration from environment variables and returns a Config.
// Returns an error if required variables are missing.
func Load() (*Config, error) {
	immudbAddr := os.Getenv("IMMUDB_ADDR")
	if immudbAddr == "" {
		return nil, fmt.Errorf("IMMUDB_ADDR is required")
	}

	immudbUsername := os.Getenv("IMMUDB_USERNAME")
	if immudbUsername == "" {
		immudbUsername = "immudb"
	}

	immudbPassword := os.Getenv("IMMUDB_PASSWORD")
	if immudbPassword == "" {
		immudbPassword = "immudb"
	}

	financeServiceAddr := os.Getenv("FINANCE_SERVICE_ADDR")
	if financeServiceAddr == "" {
		return nil, fmt.Errorf("FINANCE_SERVICE_ADDR is required")
	}

	fxServiceAddr := os.Getenv("FX_SERVICE_ADDR")
	if fxServiceAddr == "" {
		return nil, fmt.Errorf("FX_SERVICE_ADDR is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	restPort := ResolveRESTPort()

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9082"
	}

	readCacheEnabled, err := readCacheBool("EXPENSE_READ_CACHE_ENABLED", defaultReadCacheEnabled)
	if err != nil {
		return nil, err
	}
	readCacheMaxEntries, err := readCacheInt("EXPENSE_READ_CACHE_MAX_ENTRIES", defaultReadCacheMaxEntries)
	if err != nil {
		return nil, err
	}
	readCacheMaxBytes, err := readCacheInt64("EXPENSE_READ_CACHE_MAX_BYTES", defaultReadCacheMaxBytes)
	if err != nil {
		return nil, err
	}
	readCacheMaxEntryBytes, err := readCacheInt64("EXPENSE_READ_CACHE_MAX_ENTRY_BYTES", defaultReadCacheMaxEntryBytes)
	if err != nil {
		return nil, err
	}
	readCacheMaxAge, err := readCacheDuration("EXPENSE_READ_CACHE_MAX_AGE", defaultReadCacheMaxAge)
	if err != nil {
		return nil, err
	}
	financeEvictionTimeout, err := readCacheDuration("EXPENSE_FINANCE_EVICTION_TIMEOUT", defaultFinanceEvictionTimeout)
	if err != nil {
		return nil, err
	}
	if financeEvictionTimeout > financeconfig.MaxExpenseFinanceEvictionTimeout {
		return nil, fmt.Errorf("EXPENSE_FINANCE_EVICTION_TIMEOUT must be at most %s", financeconfig.MaxExpenseFinanceEvictionTimeout)
	}

	return &Config{
		ImmudbAddr:             immudbAddr,
		ImmudbUsername:         immudbUsername,
		ImmudbPassword:         immudbPassword,
		FinanceServiceAddr:     financeServiceAddr,
		FxServiceAddr:          fxServiceAddr,
		LogLevel:               logLevel,
		Environment:            environment,
		RESTPort:               restPort,
		GRPCPort:               grpcPort,
		ReadCacheEnabled:       readCacheEnabled,
		ReadCacheMaxEntries:    readCacheMaxEntries,
		ReadCacheMaxBytes:      readCacheMaxBytes,
		ReadCacheMaxEntryBytes: readCacheMaxEntryBytes,
		ReadCacheMaxAge:        readCacheMaxAge,
		FinanceEvictionTimeout: financeEvictionTimeout,
	}, nil
}

func readCacheBool(name string, defaultValue bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func readCacheInt(name string, defaultValue int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func readCacheInt64(name string, defaultValue int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func readCacheDuration(name string, defaultValue time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
