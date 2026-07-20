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

// TestResolvePort_DefaultsWhenUnset confirms the probe/listener port falls back
// to DefaultPort when PORT is not set.
func TestResolvePort_DefaultsWhenUnset(t *testing.T) {
	t.Setenv("PORT", "")
	if got := ResolvePort(); got != DefaultPort {
		t.Errorf("ResolvePort() = %q, want default %q", got, DefaultPort)
	}
}

// TestResolvePort_HonorsOverride confirms a PORT override is returned verbatim,
// so the --healthcheck probe targets the same port the listener binds.
func TestResolvePort_HonorsOverride(t *testing.T) {
	t.Setenv("PORT", "9090")
	if got := ResolvePort(); got != "9090" {
		t.Errorf("ResolvePort() = %q, want the override 9090", got)
	}
}

// TestLoad_PortDefaultsToDefaultPort proves Load sources the listener port from
// the same ResolvePort helper the probe uses, defaulting to DefaultPort.
func TestLoad_PortDefaultsToDefaultPort(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("cfg.Port = %q, want default %q", cfg.Port, DefaultPort)
	}
}

// TestLoad_PortHonorsOverride proves a PORT override reaches the listener,
// matching what the probe resolves.
func TestLoad_PortHonorsOverride(t *testing.T) {
	setGatewayEnv(t)
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("cfg.Port = %q, want the override 9090", cfg.Port)
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

// TestLoad_ValidateTimeout_WarnsAboveCeiling proves a value
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
