package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the auth service, loaded from environment variables.
type Config struct {
	DBUrl          string
	JWTSecret      string
	BcryptCost     int
	LogLevel       string
	Environment    string
	RESTPort       string
	GRPCPort       string
	CookieDomain   string
	MigrationsPath string
}

// Load reads configuration from environment variables and returns a Config.
// Returns an error if required variables are missing.
func Load() (*Config, error) {
	dbURL := os.Getenv("AUTH_DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("AUTH_DB_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	bcryptCost := 12
	if val := os.Getenv("BCRYPT_COST"); val != "" {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("BCRYPT_COST must be an integer: %w", err)
		}
		bcryptCost = parsed
		if bcryptCost < 4 || bcryptCost > 31 {
			return nil, fmt.Errorf("BCRYPT_COST must be between 4 and 31")
		}
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
		restPort = "8081"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9081"
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "/migrations"
	}

	return &Config{
		DBUrl:          dbURL,
		JWTSecret:      jwtSecret,
		BcryptCost:     bcryptCost,
		LogLevel:       logLevel,
		Environment:    environment,
		RESTPort:       restPort,
		GRPCPort:       grpcPort,
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		MigrationsPath: migrationsPath,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
