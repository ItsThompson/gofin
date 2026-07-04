package access_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// buildEngine wires AccessControl (with the canonical DefaultPolicy) in front
// of a single handler registered for method+path.
func buildEngine(validator access.TokenValidator, logger *slog.Logger, method, path string, handler gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.Use(access.AccessControl(validator, access.DefaultPolicy(), logger))
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
		{http.MethodPost, "/api/auth/onboarding-complete"}, // Personal (exact)
		{http.MethodGet, "/api/finance/periods"},           // Personal (prefix)
		{http.MethodPost, "/api/expenses"},                 // Personal (prefix)
		{http.MethodPost, "/api/datarights/exports"},       // Personal (prefix)
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
