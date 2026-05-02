package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the expense service, loaded from environment variables.
type Config struct {
	ImmudbAddr     string
	ImmudbUsername string
	ImmudbPassword string
	LogLevel       string
	Environment    string
	RESTPort       string
	GRPCPort       string
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
		restPort = "8082"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9082"
	}

	return &Config{
		ImmudbAddr:     immudbAddr,
		ImmudbUsername: immudbUsername,
		ImmudbPassword: immudbPassword,
		LogLevel:       logLevel,
		Environment:    environment,
		RESTPort:       restPort,
		GRPCPort:       grpcPort,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
