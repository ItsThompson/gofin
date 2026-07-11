package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

// DefaultRESTPort is the REST listener port used when REST_PORT is unset. It is
// also the port the --healthcheck probe targets (US-PLATFORM-05), so both the
// server and its probe share one source of truth.
const DefaultRESTPort = "8081"

const defaultGRPCPort = "9081"

// Cleanup cadence defaults for the blacklist sweep background worker.
const (
	defaultCleanupInterval = 5 * time.Minute
	defaultCleanupTimeout  = 30 * time.Second
)

// Config holds all configuration for the auth service, loaded from environment variables.
type Config struct {
	DBUrl           string
	JWTSecret       string
	BcryptCost      int
	LogLevel        string
	Environment     string
	RESTPort        string
	GRPCPort        string
	CookieDomain    string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	CleanupInterval time.Duration
	CleanupTimeout  time.Duration
}

// RESTPort returns the configured REST port from REST_PORT, or DefaultRESTPort.
// It is exported so the --healthcheck probe, which runs before the full config
// Load, targets the same port the server listens on (US-PLATFORM-05).
func RESTPort() string {
	if p := os.Getenv("REST_PORT"); p != "" {
		return p
	}
	return DefaultRESTPort
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

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = defaultGRPCPort
	}

	jwtAccessTTL, err := durationEnv("JWT_ACCESS_TTL", service.DefaultAccessTokenTTL)
	if err != nil {
		return nil, err
	}

	jwtRefreshTTL, err := durationEnv("JWT_REFRESH_TTL", service.DefaultRefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	cleanupInterval, err := durationEnv("CLEANUP_INTERVAL", defaultCleanupInterval)
	if err != nil {
		return nil, err
	}

	cleanupTimeout, err := durationEnv("CLEANUP_TIMEOUT", defaultCleanupTimeout)
	if err != nil {
		return nil, err
	}

	return &Config{
		DBUrl:           dbURL,
		JWTSecret:       jwtSecret,
		BcryptCost:      bcryptCost,
		LogLevel:        logLevel,
		Environment:     environment,
		RESTPort:        RESTPort(),
		GRPCPort:        grpcPort,
		CookieDomain:    os.Getenv("COOKIE_DOMAIN"),
		JWTAccessTTL:    jwtAccessTTL,
		JWTRefreshTTL:   jwtRefreshTTL,
		CleanupInterval: cleanupInterval,
		CleanupTimeout:  cleanupTimeout,
	}, nil
}

// durationEnv reads a time.Duration from the named env var, falling back to def
// when unset. A malformed value is a load-time error rather than a silent
// fallback so misconfiguration surfaces at boot.
func durationEnv(key string, def time.Duration) (time.Duration, error) {
	val := os.Getenv(key)
	if val == "" {
		return def, nil
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return d, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
