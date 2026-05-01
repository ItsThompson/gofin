package config

import (
	"fmt"
	"os"
)

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

	restPort := os.Getenv("REST_PORT")
	if restPort == "" {
		restPort = "8083"
	}

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
