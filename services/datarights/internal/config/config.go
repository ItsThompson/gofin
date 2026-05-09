package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the datarights service.
type Config struct {
	DBUrl           string
	LogLevel        string
	Environment     string
	RESTPort        string
	GRPCPort        string
	AuthServiceAddr string
	MaxConcurrent   int
	ExportTimeout   time.Duration
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

	authServiceAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authServiceAddr == "" {
		authServiceAddr = "auth-service:9081"
	}

	maxConcurrent := 5
	if v := os.Getenv("EXPORT_MAX_CONCURRENT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			maxConcurrent = parsed
		}
	}

	exportTimeout := 5 * time.Minute
	if v := os.Getenv("EXPORT_TIMEOUT_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			exportTimeout = time.Duration(parsed) * time.Second
		}
	}

	return &Config{
		DBUrl:           dbURL,
		LogLevel:        logLevel,
		Environment:     environment,
		RESTPort:        restPort,
		GRPCPort:        grpcPort,
		AuthServiceAddr: authServiceAddr,
		MaxConcurrent:   maxConcurrent,
		ExportTimeout:   exportTimeout,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
