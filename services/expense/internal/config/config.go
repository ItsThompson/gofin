package config

import (
	"fmt"
	"os"
)

// DefaultRESTPort is the single source of truth for the expense REST port
// default. It backs both the listener (via Load) and the --healthcheck probe
// (via ResolveRESTPort) so the two never desync.
const DefaultRESTPort = "8082"

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
	ImmudbAddr         string
	ImmudbUsername     string
	ImmudbPassword     string
	FinanceServiceAddr string
	FxServiceAddr      string
	LogLevel           string
	Environment        string
	RESTPort           string
	GRPCPort           string
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

	return &Config{
		ImmudbAddr:         immudbAddr,
		ImmudbUsername:     immudbUsername,
		ImmudbPassword:     immudbPassword,
		FinanceServiceAddr: financeServiceAddr,
		FxServiceAddr:      fxServiceAddr,
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
