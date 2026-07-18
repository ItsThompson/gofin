package access_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
)

func init() {
	// Silence gin's debug output during tests.
	gin.SetMode(gin.TestMode)
}

// fakeValidator is a TokenValidator that never touches gRPC. It records how
// many times it was called so tests can assert Public routes skip validation.
type fakeValidator struct {
	result *access.TokenValidationResult
	err    error
	calls  int
}

func (f *fakeValidator) ValidateToken(_ context.Context, _ string) (*access.TokenValidationResult, error) {
	f.calls++
	return f.result, f.err
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// captureLogger returns a JSON logger and the buffer it writes to, so tests can
// assert on the structured fields emitted on 401/403.
func captureLogger() (*slog.Logger, *bytesBuffer) {
	buf := &bytesBuffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return logger, buf
}

// buildEngine wires AccessControl (with the gateway's GatewayResolve, which
// classifies every route via the shared services/access registry) in front of
// a single handler registered for method+path.
func buildEngine(validator access.TokenValidator, logger *slog.Logger, method, path string, handler gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.Use(access.AccessControl(validator, access.GatewayResolve, logger))
	engine.Handle(method, path, handler)
	return engine
}

func okHandler(c *gin.Context) { c.Status(http.StatusOK) }

// --- Matrix: {Public, Authenticated, Personal, Admin} x {no-token, user, admin, assumed-user} ---

type tokenScenario struct {
	name         string
	cookie       bool
	newValidator func() *fakeValidator
}

func tokenScenarios() []tokenScenario {
	return []tokenScenario{
		{
			name:   "no-token",
			cookie: false,
			// Errors if reached; a cookie-less request must 401 before validation.
			newValidator: func() *fakeValidator {
				return &fakeValidator{err: errors.New("validate should not be called without a cookie")}
			},
		},
		{
			name:   "user",
			cookie: true,
			newValidator: func() *fakeValidator {
				return &fakeValidator{result: &access.TokenValidationResult{UserID: "user-1", Role: "user"}}
			},
		},
		{
			name:   "admin",
			cookie: true,
			newValidator: func() *fakeValidator {
				return &fakeValidator{result: &access.TokenValidationResult{UserID: "admin-1", Role: "admin"}}
			},
		},
		{
			name:   "assumed-user",
			cookie: true,
			newValidator: func() *fakeValidator {
				return &fakeValidator{result: &access.TokenValidationResult{UserID: "target-1", Role: "user", AssumedBy: "admin-1"}}
			},
		},
	}
}

func TestAccessControl_Matrix(t *testing.T) {
	routes := []struct {
		level  string
		method string
		path   string
	}{
		{"Public", http.MethodPost, "/api/auth/login"},
		{"Authenticated", http.MethodPost, "/api/auth/restore"},
		{"Personal", http.MethodGet, "/api/finance/periods"},
		{"Admin", http.MethodGet, "/api/admin/users"},
	}

	// Expected status per (access level x token scenario).
	want := map[string]map[string]int{
		"Public":        {"no-token": 200, "user": 200, "admin": 200, "assumed-user": 200},
		"Authenticated": {"no-token": 401, "user": 200, "admin": 200, "assumed-user": 200},
		"Personal":      {"no-token": 401, "user": 200, "admin": 403, "assumed-user": 200},
		"Admin":         {"no-token": 401, "user": 403, "admin": 200, "assumed-user": 403},
	}

	for _, route := range routes {
		for _, sc := range tokenScenarios() {
			t.Run(route.level+"/"+sc.name, func(t *testing.T) {
				validator := sc.newValidator()
				engine := buildEngine(validator, silentLogger(), route.method, route.path, okHandler)

				req := httptest.NewRequest(route.method, route.path, nil)
				if sc.cookie {
					req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
				}
				rec := httptest.NewRecorder()
				engine.ServeHTTP(rec, req)

				assert.Equal(t, want[route.level][sc.name], rec.Code,
					"%s route with %s token", route.level, sc.name)
			})
		}
	}
}

// --- Public routes: no cookie read, no validation ---

func TestAccessControl_Public_SkipsValidationAndStripsHeaders(t *testing.T) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "should-not-appear", Role: "admin"},
	}

	var userID, role, assumedBy string
	engine := buildEngine(validator, silentLogger(), http.MethodPost, "/api/auth/login", func(c *gin.Context) {
		userID = c.Request.Header.Get("X-User-ID")
		role = c.Request.Header.Get("X-User-Role")
		assumedBy = c.Request.Header.Get("X-Assumed-By")
		c.Status(http.StatusOK)
	})

	// A cookie is present and identity headers are spoofed: neither should matter.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-Assumed-By", "spoofed-admin")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, validator.calls, "Public routes must not call ValidateToken")
	assert.Empty(t, userID, "spoofed X-User-ID must be stripped on Public routes")
	assert.Empty(t, role, "spoofed X-User-Role must be stripped on Public routes")
	assert.Empty(t, assumedBy, "spoofed X-Assumed-By must be stripped on Public routes")
}

// --- Anti-spoof: inbound identity headers replaced by validated identity ---

func TestAccessControl_StripsSpoofedHeaders_UsesValidatedIdentity(t *testing.T) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "user-1", Role: "user"},
	}

	var userID, role, assumedBy string
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/finance/periods", func(c *gin.Context) {
		userID = c.Request.Header.Get("X-User-ID")
		role = c.Request.Header.Get("X-User-Role")
		assumedBy = c.Request.Header.Get("X-Assumed-By")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	req.Header.Set("X-User-ID", "spoofed-admin")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-Assumed-By", "spoofed-admin")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-1", userID, "must forward validated user id, not spoofed")
	assert.Equal(t, "user", role, "must forward validated role, not spoofed admin")
	assert.Empty(t, assumedBy, "X-Assumed-By must be cleared when the token is not assumed")
}

// --- On pass: downstream headers + gin context for RequestLogger ---

func TestAccessControl_Pass_SetsDownstreamIdentityAndContext(t *testing.T) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "user-1", Role: "user"},
	}

	var headerUserID, contextUserID string
	engine := buildEngine(validator, silentLogger(), http.MethodPost, "/api/auth/restore", func(c *gin.Context) {
		headerUserID = c.Request.Header.Get("X-User-ID")
		if v, ok := c.Get("X-User-ID"); ok {
			contextUserID, _ = v.(string)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/restore", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-1", headerUserID, "X-User-ID header set downstream")
	assert.Equal(t, "user-1", contextUserID, "X-User-ID set in gin context for RequestLogger")
}

// --- Assumed session: X-Assumed-By forwarded, reaches Personal + restore ---

func TestAccessControl_ForwardsAssumedBy(t *testing.T) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "target-1", Role: "user", AssumedBy: "admin-1"},
	}

	var assumedBy string
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/finance/periods", func(c *gin.Context) {
		assumedBy = c.Request.Header.Get("X-Assumed-By")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "admin-1", assumedBy, "X-Assumed-By forwarded for an assumed session")
}

func TestAccessControl_AssumedUser_PassesPersonalRoutesAndRestore(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/onboarding-complete"}, // Personal
		{http.MethodGet, "/api/finance/periods"},           // Personal
		{http.MethodPost, "/api/expenses"},                 // Personal
		{http.MethodPost, "/api/datarights/exports"},       // Personal
		{http.MethodPost, "/api/auth/restore"},             // Authenticated
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			validator := &fakeValidator{
				result: &access.TokenValidationResult{UserID: "target-1", Role: "user", AssumedBy: "admin-1"},
			}
			engine := buildEngine(validator, silentLogger(), route.method, route.path, okHandler)

			req := httptest.NewRequest(route.method, route.path, nil)
			req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

// --- 401 paths ---

func TestAccessControl_EmptyCookie_Returns401(t *testing.T) {
	validator := &fakeValidator{err: errors.New("should not be called for an empty cookie")}
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/auth/me", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: ""})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "UNAUTHORIZED")
	assert.Equal(t, 0, validator.calls)
}

func TestAccessControl_ValidationError_Returns401(t *testing.T) {
	validator := &fakeValidator{err: errors.New("token expired")}
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/auth/me", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "expired-token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "UNAUTHORIZED")
}

// --- 503 timeout mapping: a bounded-validation deadline is not a bad token ---

// TestAccessControl_ValidationTimeout_Returns503 pins the key semantic split:
// a ValidateToken error that is (or wraps) context.DeadlineExceeded, or that
// carries a gRPC DeadlineExceeded status, maps to 503 SERVICE_UNAVAILABLE (the
// auth dependency is unhealthy), distinct from the 401 for a genuine rejection.
// Both detection prongs and the fmt.Errorf %w wrap the concrete validator adds
// are covered.
func TestAccessControl_ValidationTimeout_Returns503(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"raw context deadline", context.DeadlineExceeded},
		{"wrapped context deadline", fmt.Errorf("auth service validation failed: %w", context.DeadlineExceeded)},
		{"grpc deadline status", status.Error(codes.DeadlineExceeded, "context deadline exceeded")},
		{"wrapped grpc deadline status", fmt.Errorf("auth service validation failed: %w", status.Error(codes.DeadlineExceeded, "context deadline exceeded"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &fakeValidator{err: tc.err}
			engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/finance/periods", okHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
			req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "a validation deadline must map to 503")
			assert.Contains(t, rec.Body.String(), "SERVICE_UNAVAILABLE")
			assert.NotContains(t, rec.Body.String(), "UNAUTHORIZED", "a timeout must not be reported as an invalid token")
		})
	}
}

// TestAccessControl_NonDeadlineGRPCError_Returns401 proves only the deadline is
// special-cased: every other gRPC status (here Unauthenticated) still returns
// the unchanged 401 contract.
func TestAccessControl_NonDeadlineGRPCError_Returns401(t *testing.T) {
	validator := &fakeValidator{err: status.Error(codes.Unauthenticated, "invalid token")}
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/finance/periods", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "UNAUTHORIZED")
}

// TestAccessControl_ValidationTimeout_LogsWarningNamingAuth asserts the 503 path
// emits a distinct warn log that names the auth dependency as the cause, so the
// timeout is diagnosable separately from a 401.
func TestAccessControl_ValidationTimeout_LogsWarningNamingAuth(t *testing.T) {
	logger, buf := captureLogger()
	validator := &fakeValidator{err: status.Error(codes.DeadlineExceeded, "context deadline exceeded")}
	engine := buildEngine(validator, logger, http.MethodGet, "/api/finance/periods", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	entry := buf.lastEntry(t)
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "auth validation timed out", entry["msg"])
	assert.Equal(t, "auth", entry["dependency"])
	assert.Equal(t, "/api/finance/periods", entry["path"])
}

// --- Warn logging on 401 and 403 ---

func TestAccessControl_Unauthorized_LogsWarning(t *testing.T) {
	logger, buf := captureLogger()
	validator := &fakeValidator{err: errors.New("token expired")}
	engine := buildEngine(validator, logger, http.MethodGet, "/api/finance/periods", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "expired-token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	entry := buf.lastEntry(t)
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "GET", entry["method"])
	assert.Equal(t, "/api/finance/periods", entry["path"])
	assert.Equal(t, "token expired", entry["error"])
}

func TestAccessControl_Forbidden_LogsWarning(t *testing.T) {
	logger, buf := captureLogger()
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "user-1", Role: "user"},
	}
	engine := buildEngine(validator, logger, http.MethodGet, "/api/admin/users", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "FORBIDDEN")

	entry := buf.lastEntry(t)
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "GET", entry["method"])
	assert.Equal(t, "/api/admin/users", entry["path"])
	assert.Equal(t, "user", entry["role"])
	assert.Equal(t, "user-1", entry["user_id"])
}

// --- Fail-safe: an unrecognized access level is denied by construction ---

func TestAccessControl_UnknownLevel_DeniesByDefault(t *testing.T) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "user-1", Role: "user"},
	}

	// A resolve func that returns an out-of-enum access level for every path.
	// A valid token reaches the middleware's switch with a level that matches
	// none of the known cases and must fall through to the fail-safe deny.
	resolve := func(_, _ string) sharedaccess.Level { return sharedaccess.Level(99) }

	engine := gin.New()
	engine.Use(access.AccessControl(validator, resolve, silentLogger()))
	engine.GET("/api/anything", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "unknown access level must be denied")
	assert.Contains(t, rec.Body.String(), "FORBIDDEN")
}

// TestAccessControl_Deny_NoCookie_Returns403 proves the deny-by-default
// short-circuit: an unclassified /api path (no Registry entry) resolves to Deny
// via GatewayResolve, and the middleware must 403 before any cookie is read.
// An unclassified route is not a real route, so no identity is required and the
// token validator must not be called.
func TestAccessControl_Deny_NoCookie_Returns403(t *testing.T) {
	validator := &fakeValidator{err: errors.New("validate must not be called for a denied route")}
	engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/unclassified", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/unclassified", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "an unclassified route must be denied")
	assert.Contains(t, rec.Body.String(), "FORBIDDEN")
	assert.Equal(t, 0, validator.calls, "a denied route must not read the cookie or validate a token")
}

// --- Gateway-native classification: /health, /metrics, and unknown paths ---

// TestGatewayResolve covers the gateway-owned classification that services/access
// intentionally does not know about: the gateway-native /health and /metrics
// endpoints are Public, while every other path is delegated to the shared
// registry resolver (a real route keeps its level; an unknown path falls to the
// fail-safe Deny default).
func TestGatewayResolve(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   sharedaccess.Level
	}{
		{"health is public", http.MethodGet, "/health", sharedaccess.Public},
		{"metrics is public", http.MethodGet, "/metrics", sharedaccess.Public},
		{"readyz is public", http.MethodGet, "/readyz", sharedaccess.Public},
		{"a real personal route keeps its level", http.MethodGet, "/api/finance/periods", sharedaccess.Personal},
		{"a real admin route keeps its level", http.MethodGet, "/api/admin/users", sharedaccess.Admin},
		{"an unknown path falls to deny", http.MethodGet, "/api/unknown", sharedaccess.Deny},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := access.GatewayResolve(tc.method, tc.path); got != tc.want {
				t.Errorf("GatewayResolve(%q, %q) = %s, want %s", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// TestAccessControl_ErrorBodies_MatchApierrWireShape pins that routing the
// gateway's middleware errors through apierr.Respond preserves the exact
// {code,message} wire bytes clients already receive, so the migration is not a
// schema change. Covers the 401 (missing cookie), 403 (denied route), and 503
// (auth-dependency timeout) paths. Asserts the raw body (not JSONEq) so the
// byte-for-byte claim is pinned: field order and the absence of a trailing
// newline are part of the contract.
func TestAccessControl_ErrorBodies_MatchApierrWireShape(t *testing.T) {
	t.Run("401 missing cookie", func(t *testing.T) {
		engine := buildEngine(&fakeValidator{}, silentLogger(), http.MethodGet, "/api/auth/me", okHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, `{"code":"UNAUTHORIZED","message":"Authentication required"}`, rec.Body.String())
	})

	t.Run("403 denied unclassified route", func(t *testing.T) {
		engine := buildEngine(&fakeValidator{}, silentLogger(), http.MethodGet, "/api/unclassified", okHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/unclassified", nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, `{"code":"FORBIDDEN","message":"Access denied"}`, rec.Body.String())
	})

	t.Run("503 auth dependency timeout", func(t *testing.T) {
		validator := &fakeValidator{err: context.DeadlineExceeded}
		engine := buildEngine(validator, silentLogger(), http.MethodGet, "/api/finance/periods", okHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
		req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, `{"code":"SERVICE_UNAVAILABLE","message":"Authentication service unavailable"}`, rec.Body.String())
	})
}

// bytesBuffer is a minimal io.Writer that also parses the last JSON log line.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

// lastEntry parses the final newline-delimited JSON log line into a map.
func (b *bytesBuffer) lastEntry(t *testing.T) map[string]any {
	t.Helper()
	lines := splitNonEmptyLines(b.data)
	require.NotEmpty(t, lines, "expected at least one log line")

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &entry))
	return entry
}

func splitNonEmptyLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, string(data[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}
