package config

import (
	"log/slog"
	"strings"
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

// captureDefaultLogger swaps slog's default logger for one writing to the
// returned buffer (Load emits its oversized-value warning through slog.Warn,
// i.e. the default logger) and restores it on cleanup.
func captureDefaultLogger(t *testing.T) *strings.Builder {
	t.Helper()
	buf := &strings.Builder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// TestLoad_ValidateTimeout_WarnsAboveCeiling proves the GW-4 soft bound: a value
// above the recommended ceiling is accepted unchanged (no hard clamp) but logs
// a visible warning so a "1h"-style typo that defeats the backstop is caught.
func TestLoad_ValidateTimeout_WarnsAboveCeiling(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("GATEWAY_VALIDATE_TIMEOUT", "45s")
	buf := captureDefaultLogger(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ValidateTimeout != 45*time.Second {
		t.Errorf("ValidateTimeout = %v, want 45s (accepted, no hard clamp)", cfg.ValidateTimeout)
	}
	if !strings.Contains(buf.String(), "unusually large") {
		t.Errorf("expected a warning for a value above the ceiling, got %q", buf.String())
	}
}

// TestLoad_ValidateTimeout_NoWarnAtCeiling confirms the warning does not fire at
// or below the recommended ceiling.
func TestLoad_ValidateTimeout_NoWarnAtCeiling(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("GATEWAY_VALIDATE_TIMEOUT", "30s")
	buf := captureDefaultLogger(t)

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if strings.Contains(buf.String(), "unusually large") {
		t.Errorf("did not expect a warning at the recommended ceiling, got %q", buf.String())
	}
}
