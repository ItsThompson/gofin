package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the datarights service.
type Config struct {
	DBUrl       string
	LogLevel    string
	Environment string
	RESTPort    string
	GRPCPort    string
}

// Load reads configuration from environment variables and returns a Config.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATARIGHTS_DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATARIGHTS_DB_URL is required")
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
		restPort = "8084"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9084"
	}

	return &Config{
		DBUrl:       dbURL,
		LogLevel:    logLevel,
		Environment: environment,
		RESTPort:    restPort,
		GRPCPort:    grpcPort,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
