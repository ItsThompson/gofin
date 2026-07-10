package config

import (
	"testing"
	"time"
)

// setGatewayEnv sets the minimum required env vars so Load reaches the
// GATEWAY_VALIDATE_TIMEOUT parsing under test. t.Setenv restores them after.
func setGatewayEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_SERVICE_ADDR", "auth-service:9081")
	t.Setenv("AUTH_SERVICE_REST", "http://auth-service:8081")
	t.Setenv("EXPENSE_SERVICE_REST", "http://expense-service:8082")
	t.Setenv("FINANCE_SERVICE_REST", "http://finance-service:8083")
	t.Setenv("DATARIGHTS_SERVICE_REST", "http://datarights-service:8084")
}

func TestLoad_ValidateTimeout_DefaultsWhenUnset(t *testing.T) {
	setGatewayEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ValidateTimeout != 3*time.Second {
		t.Errorf("ValidateTimeout = %v, want default 3s", cfg.ValidateTimeout)
	}
}

func TestLoad_ValidateTimeout_ParsesOverride(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("GATEWAY_VALIDATE_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ValidateTimeout != 5*time.Second {
		t.Errorf("ValidateTimeout = %v, want 5s", cfg.ValidateTimeout)
	}
}

func TestLoad_ValidateTimeout_RejectsUnparseable(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("GATEWAY_VALIDATE_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for unparseable GATEWAY_VALIDATE_TIMEOUT")
	}
}

func TestLoad_ValidateTimeout_RejectsNonPositive(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("GATEWAY_VALIDATE_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for non-positive GATEWAY_VALIDATE_TIMEOUT")
	}
}
