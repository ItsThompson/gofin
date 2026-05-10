package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the datarights service.
type Config struct {
	DBUrl              string
	LogLevel           string
	Environment        string
	RESTPort           string
	GRPCPort           string
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

	return &Config{
		DBUrl:              dbURL,
		LogLevel:           logLevel,
		Environment:        environment,
		RESTPort:           restPort,
		GRPCPort:           grpcPort,
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
	}, nil
}

// IsProduction returns true if the environment is not "development".
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}
