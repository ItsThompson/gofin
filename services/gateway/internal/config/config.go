package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// defaultValidateTimeout is a hung-dependency backstop, not p99/tail-latency
// control: 3s comfortably exceeds a healthy ValidateToken so it only fires when
// the dependency is effectively hung. Override with GATEWAY_VALIDATE_TIMEOUT.
const defaultValidateTimeout = 3 * time.Second

// maxRecommendedValidateTimeout is the largest GATEWAY_VALIDATE_TIMEOUT that
// still serves as a hang backstop. Larger values are accepted (there is no hard
// clamp) but warned about at load: a "1h" typo would silently re-introduce the
// near-unbounded tail this bound exists to prevent.
const maxRecommendedValidateTimeout = 30 * time.Second

// DefaultPort is the gateway's default HTTP listen port. It is the single
// source of truth shared by Load (the listener) and the --healthcheck probe so
// the two can never desync.
const DefaultPort = "8080"

// Config holds all configuration for the API gateway, loaded from environment variables.
type Config struct {
	AuthServiceAddr       string // gRPC address for auth service (e.g., "auth-service:9081")
	AuthServiceREST       string // REST base URL for auth service (e.g., "http://auth-service:8081")
	ExpenseServiceREST    string // REST base URL for expense service
	FinanceServiceREST    string // REST base URL for finance service
	DatarightsServiceREST string // REST base URL for datarights service
	LogLevel              string
	Environment           string
	Port                  string
	ValidateTimeout       time.Duration // upper bound on the ValidateToken gRPC call (GATEWAY_VALIDATE_TIMEOUT, default 3s)
}

// Load reads configuration from environment variables and returns a Config.
// Returns an error if required variables are missing.
func Load() (*Config, error) {
	authAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authAddr == "" {
		return nil, fmt.Errorf("AUTH_SERVICE_ADDR is required")
	}

	authREST := os.Getenv("AUTH_SERVICE_REST")
	if authREST == "" {
		return nil, fmt.Errorf("AUTH_SERVICE_REST is required")
	}

	expenseREST := os.Getenv("EXPENSE_SERVICE_REST")
	if expenseREST == "" {
		return nil, fmt.Errorf("EXPENSE_SERVICE_REST is required")
	}

	financeREST := os.Getenv("FINANCE_SERVICE_REST")
	if financeREST == "" {
		return nil, fmt.Errorf("FINANCE_SERVICE_REST is required")
	}

	datarightsREST := os.Getenv("DATARIGHTS_SERVICE_REST")
	if datarightsREST == "" {
		return nil, fmt.Errorf("DATARIGHTS_SERVICE_REST is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	port := ResolvePort()

	validateTimeout := defaultValidateTimeout
	if raw := os.Getenv("GATEWAY_VALIDATE_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing GATEWAY_VALIDATE_TIMEOUT %q: %w", raw, err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("GATEWAY_VALIDATE_TIMEOUT must be positive, got %q", raw)
		}
		if parsed > maxRecommendedValidateTimeout {
			slog.Warn("GATEWAY_VALIDATE_TIMEOUT is unusually large; it is a hung-dependency backstop, not a normal latency bound",
				slog.Duration("configured", parsed),
				slog.Duration("recommended_ceiling", maxRecommendedValidateTimeout),
			)
		}
		validateTimeout = parsed
	}

	return &Config{
		AuthServiceAddr:       authAddr,
		AuthServiceREST:       authREST,
		ExpenseServiceREST:    expenseREST,
		FinanceServiceREST:    financeREST,
		DatarightsServiceREST: datarightsREST,
		LogLevel:              logLevel,
		Environment:           environment,
		Port:                  port,
		ValidateTimeout:       validateTimeout,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}

// ResolvePort returns the configured HTTP port: the PORT env override when set,
// else DefaultPort. Both the listener (via Load) and the --healthcheck probe
// call this, so overriding PORT moves them together instead of desyncing the
// probe from a hardcoded literal. It is standalone (not a
// Config method) so the pre-config --healthcheck branch can call it without a
// full Load.
func ResolvePort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return DefaultPort
}
