package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultAuthAddr          = "auth-service:9081"
	DefaultExpenseAddr       = "expense-service:9082"
	DefaultFinanceAddr       = "finance-service:9083"
	DefaultDatarightsAddr    = "datarights-service:9084"
	DefaultRPCTimeout        = 10 * time.Second
	DefaultCollectionTimeout = 15 * time.Second
	DefaultDiscordTimeout    = 10 * time.Second
	DefaultDeliveryTimeout   = 60 * time.Second
	DefaultAppTimeout        = 90 * time.Second
)

type Config struct {
	AuthAddr          string
	ExpenseAddr       string
	FinanceAddr       string
	DatarightsAddr    string
	DiscordWebhookURL string
	RPCTimeout        time.Duration
	CollectionTimeout time.Duration
	DiscordTimeout    time.Duration
	DeliveryTimeout   time.Duration
	AppTimeout        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AuthAddr:          envOrDefault("AUTH_SERVICE_ADDR", DefaultAuthAddr),
		ExpenseAddr:       envOrDefault("EXPENSE_SERVICE_ADDR", DefaultExpenseAddr),
		FinanceAddr:       envOrDefault("FINANCE_SERVICE_ADDR", DefaultFinanceAddr),
		DatarightsAddr:    envOrDefault("DATARIGHTS_SERVICE_ADDR", DefaultDatarightsAddr),
		DiscordWebhookURL: strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
		RPCTimeout:        DefaultRPCTimeout,
		CollectionTimeout: DefaultCollectionTimeout,
		DiscordTimeout:    DefaultDiscordTimeout,
		DeliveryTimeout:   DefaultDeliveryTimeout,
		AppTimeout:        DefaultAppTimeout,
	}
	if cfg.AuthAddr == "" || cfg.ExpenseAddr == "" || cfg.FinanceAddr == "" || cfg.DatarightsAddr == "" {
		return Config{}, fmt.Errorf("service addresses must not be empty")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func ParseReportDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if len(value) != len("2006-01-02") {
		return time.Time{}, fmt.Errorf("report date must be YYYY-MM-DD")
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("report date must be YYYY-MM-DD")
	}
	if parsed.Weekday() != time.Monday {
		return time.Time{}, fmt.Errorf("report date must be a Monday")
	}
	return parsed, nil
}

func LatestCompletedMonday(now time.Time) time.Time {
	utc := now.UTC()
	midnight := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	daysSinceMonday := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -daysSinceMonday-7)
}

func ResolveReportDate(value string, now time.Time) (time.Time, error) {
	parsed, err := ParseReportDate(value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed.IsZero() {
		return LatestCompletedMonday(now), nil
	}
	if parsed.AddDate(0, 0, 7).After(now.UTC()) {
		return time.Time{}, fmt.Errorf("report week must be completed")
	}
	return parsed, nil
}
