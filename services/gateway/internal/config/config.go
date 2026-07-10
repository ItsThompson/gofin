package config

import (
	"fmt"
	"os"
	"time"
)

// defaultValidateTimeout bounds the gateway's ValidateToken gRPC call so a hung
// auth service returns a fast error instead of blocking the worker
// indefinitely. The default sits in the audit's suggested 2-5s band: it must
// comfortably exceed a healthy ValidateToken (a JWT check plus a DB lookup, low
// milliseconds) so it never trips in normal operation, while capping a hang
// well under a human's patience. Override with GATEWAY_VALIDATE_TIMEOUT.
const defaultValidateTimeout = 3 * time.Second

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	validateTimeout := defaultValidateTimeout
	if raw := os.Getenv("GATEWAY_VALIDATE_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing GATEWAY_VALIDATE_TIMEOUT %q: %w", raw, err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("GATEWAY_VALIDATE_TIMEOUT must be positive, got %q", raw)
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
