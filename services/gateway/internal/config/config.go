package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the API gateway, loaded from environment variables.
type Config struct {
	AuthServiceAddr    string // gRPC address for auth service (e.g., "auth-service:9081")
	AuthServiceREST    string // REST base URL for auth service (e.g., "http://auth-service:8081")
	ExpenseServiceREST string // REST base URL for expense service
	FinanceServiceREST string // REST base URL for finance service
	LogLevel           string
	Environment        string
	Port               string
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

	return &Config{
		AuthServiceAddr:    authAddr,
		AuthServiceREST:    authREST,
		ExpenseServiceREST: expenseREST,
		FinanceServiceREST: financeREST,
		LogLevel:           logLevel,
		Environment:        environment,
		Port:               port,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
