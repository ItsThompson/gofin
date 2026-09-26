package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	financeconfig "github.com/ItsThompson/gofin/services/finance/config"
)

// DefaultRESTPort is the single source of truth for the finance REST port
// default. It backs both the listener (via Load) and the --healthcheck probe
// (via ResolveRESTPort) so the two never desync.
const DefaultRESTPort = "8083"

// ReportDomain tags every error report this service makes; it is a query
// dimension shared across services so cross-project Sentry queries work.
// Budgets, periods, tags, and spending are one business area, hence a single
// domain. The closed set is documented in docs/error-handling.md.
const ReportDomain = "budgets"

const (
	defaultResultCacheEnabled       = false
	defaultResultCacheMaxEntries    = 256
	defaultResultCacheMaxBytes      = 64 * 1024 * 1024
	defaultResultCacheMaxEntryBytes = 16 * 1024 * 1024
	defaultResultCacheMaxAge        = 48 * time.Hour
	defaultResultCacheLease         = financeconfig.FinanceValidationLease
	defaultResultCacheTimeout       = financeconfig.FinanceValidationTimeout
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

// Config holds all configuration for the finance service, loaded from environment variables.
type Config struct {
	DBUrl                    string
	ExpenseServiceAddr       string // gRPC address for expense service (e.g., "expense-service:9082")
	FxServiceAddr            string // gRPC address for fx service (e.g., "fx-service:9085")
	LogLevel                 string
	Environment              string
	RESTPort                 string
	GRPCPort                 string
	ResultCacheEnabled       bool
	ResultCacheMaxEntries    int
	ResultCacheMaxBytes      int64
	ResultCacheMaxEntryBytes int64
	ResultCacheMaxAge        time.Duration
	ResultCacheLease         time.Duration
	ResultCacheTimeout       time.Duration
}

// Load reads configuration from environment variables and returns a Config.
// Returns an error if required variables are missing.
func Load() (*Config, error) {
	dbURL := os.Getenv("FINANCE_DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("FINANCE_DB_URL is required")
	}

	expenseAddr := os.Getenv("EXPENSE_SERVICE_ADDR")
	if expenseAddr == "" {
		return nil, fmt.Errorf("EXPENSE_SERVICE_ADDR is required")
	}

	fxAddr := os.Getenv("FX_SERVICE_ADDR")
	if fxAddr == "" {
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
		grpcPort = "9083"
	}

	resultCacheEnabled, err := readCacheBool("FINANCE_RESULT_CACHE_ENABLED", defaultResultCacheEnabled)
	if err != nil {
		return nil, err
	}
	resultCacheMaxEntries, err := readCacheInt("FINANCE_RESULT_CACHE_MAX_ENTRIES", defaultResultCacheMaxEntries)
	if err != nil {
		return nil, err
	}
	resultCacheMaxBytes, err := readCacheInt64("FINANCE_RESULT_CACHE_MAX_BYTES", defaultResultCacheMaxBytes)
	if err != nil {
		return nil, err
	}
	if resultCacheMaxBytes > financeconfig.MaxFinanceResultCacheBytes {
		return nil, fmt.Errorf("FINANCE_RESULT_CACHE_MAX_BYTES must be at most %d", financeconfig.MaxFinanceResultCacheBytes)
	}
	resultCacheMaxEntryBytes, err := readCacheInt64("FINANCE_RESULT_CACHE_MAX_ENTRY_BYTES", defaultResultCacheMaxEntryBytes)
	if err != nil {
		return nil, err
	}
	resultCacheMaxAge, err := readCacheDuration("FINANCE_RESULT_CACHE_MAX_AGE", defaultResultCacheMaxAge)
	if err != nil {
		return nil, err
	}
	resultCacheLease, err := readCacheDuration("FINANCE_RESULT_CACHE_VALIDATION_LEASE", defaultResultCacheLease)
	if err != nil {
		return nil, err
	}
	if resultCacheLease <= financeconfig.MaxExpenseFinanceEvictionTimeout {
		return nil, fmt.Errorf("FINANCE_RESULT_CACHE_VALIDATION_LEASE must be greater than %s", financeconfig.MaxExpenseFinanceEvictionTimeout)
	}
	resultCacheTimeout, err := readCacheDuration("FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT", defaultResultCacheTimeout)
	if err != nil {
		return nil, err
	}
	if resultCacheTimeout >= resultCacheLease {
		return nil, fmt.Errorf("FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT must be less than FINANCE_RESULT_CACHE_VALIDATION_LEASE")
	}

	return &Config{
		DBUrl:                    dbURL,
		ExpenseServiceAddr:       expenseAddr,
		FxServiceAddr:            fxAddr,
		LogLevel:                 logLevel,
		Environment:              environment,
		RESTPort:                 restPort,
		GRPCPort:                 grpcPort,
		ResultCacheEnabled:       resultCacheEnabled,
		ResultCacheMaxEntries:    resultCacheMaxEntries,
		ResultCacheMaxBytes:      resultCacheMaxBytes,
		ResultCacheMaxEntryBytes: resultCacheMaxEntryBytes,
		ResultCacheMaxAge:        resultCacheMaxAge,
		ResultCacheLease:         resultCacheLease,
		ResultCacheTimeout:       resultCacheTimeout,
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
