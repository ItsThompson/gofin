package config

import (
	"fmt"
	"os"
)

// DefaultRESTPort is the single source of truth for the finance REST port
// default. It backs both the listener (via Load) and the --healthcheck probe
// (via ResolveRESTPort) so the two never desync (US-PLATFORM-05).
const DefaultRESTPort = "8083"

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
	DBUrl              string
	ExpenseServiceAddr string // gRPC address for expense service (e.g., "expense-service:9082")
	LogLevel           string
	Environment        string
	RESTPort           string
	GRPCPort           string
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

	return &Config{
		DBUrl:              dbURL,
		ExpenseServiceAddr: expenseAddr,
		LogLevel:           logLevel,
		Environment:        environment,
		RESTPort:           restPort,
		GRPCPort:           grpcPort,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
