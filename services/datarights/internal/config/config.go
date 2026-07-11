package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultProtectedUsernames is the fallback protected-username list used when
// PROTECTED_USERNAMES is unset. These accounts cannot be deleted via the
// datarights deletion flow. The check itself is owned by the datarights
// service (moved from auth).
var DefaultProtectedUsernames = []string{"admin", "thompson"}

// Config holds all configuration for the datarights service.
type Config struct {
	DBUrl              string
	LogLevel           string
	Environment        string
	RESTPort           string
	AuthServiceAddr    string
	ExpenseServiceAddr string
	FinanceServiceAddr string
	MaxConcurrent      int
	ExportTimeout      time.Duration
	DeletionTimeout    time.Duration
	ResendAPIKey       string
	EmailFrom          string
	EmailEnabled       bool
	BrandTokensPath    string
	ProtectedUsernames []string
}

// DefaultRESTPort is the datarights REST listener port used when REST_PORT is
// unset. It is the single source of truth shared by the server listener and the
// --healthcheck probe so a REST_PORT override never desyncs them (US-PLATFORM-05).
const DefaultRESTPort = "8084"

// RESTPort returns the configured REST port, honoring REST_PORT with
// DefaultRESTPort as the fallback. Both Load (the listener) and the
// --healthcheck probe call it.
func RESTPort() string {
	if port := os.Getenv("REST_PORT"); port != "" {
		return port
	}
	return DefaultRESTPort
}

// parseProtectedUsernames splits a comma-separated PROTECTED_USERNAMES value
// into a trimmed, non-empty list. When the value is empty or yields no
// usernames it returns a fresh copy of DefaultProtectedUsernames, so callers
// never share (and cannot mutate) the package-level default slice.
func parseProtectedUsernames(raw string) []string {
	var names []string
	for _, part := range strings.Split(raw, ",") {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return append([]string(nil), DefaultProtectedUsernames...)
	}
	return names
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

	restPort := RESTPort()

	authServiceAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authServiceAddr == "" {
		authServiceAddr = "auth-service:9081"
	}

	expenseServiceAddr := os.Getenv("EXPENSE_SERVICE_ADDR")
	if expenseServiceAddr == "" {
		expenseServiceAddr = "expense-service:9082"
	}

	financeServiceAddr := os.Getenv("FINANCE_SERVICE_ADDR")
	if financeServiceAddr == "" {
		financeServiceAddr = "finance-service:9083"
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

	deletionTimeout := 5 * time.Minute
	if v := os.Getenv("DELETION_TIMEOUT_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			deletionTimeout = time.Duration(parsed) * time.Second
		}
	}

	emailEnabled := true
	if v := os.Getenv("EMAIL_ENABLED"); v == "false" {
		emailEnabled = false
	}

	resendAPIKey := os.Getenv("RESEND_API_KEY")
	emailFrom := os.Getenv("EMAIL_FROM")
	if emailFrom == "" {
		emailFrom = "gofin <noreply@usegofin.com>"
	}

	brandTokensPath := os.Getenv("BRAND_TOKENS_PATH")
	if brandTokensPath == "" {
		brandTokensPath = "/app/tokens/brand.json"
	}

	protectedUsernames := parseProtectedUsernames(os.Getenv("PROTECTED_USERNAMES"))

	return &Config{
		DBUrl:              dbURL,
		LogLevel:           logLevel,
		Environment:        environment,
		RESTPort:           restPort,
		AuthServiceAddr:    authServiceAddr,
		ExpenseServiceAddr: expenseServiceAddr,
		FinanceServiceAddr: financeServiceAddr,
		MaxConcurrent:      maxConcurrent,
		ExportTimeout:      exportTimeout,
		DeletionTimeout:    deletionTimeout,
		ResendAPIKey:       resendAPIKey,
		EmailFrom:          emailFrom,
		EmailEnabled:       emailEnabled,
		BrandTokensPath:    brandTokensPath,
		ProtectedUsernames: protectedUsernames,
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
