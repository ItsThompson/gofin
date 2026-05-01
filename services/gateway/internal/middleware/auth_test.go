package middleware_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
)

// mockTokenValidator implements middleware.TokenValidator for tests.
type mockTokenValidator struct {
	result *middleware.TokenValidationResult
	err    error
}

func (m *mockTokenValidator) ValidateToken(_ context.Context, _ string) (*middleware.TokenValidationResult, error) {
	return m.result, m.err
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAuth_ValidToken_InjectsHeaders(t *testing.T) {
	validator := &mockTokenValidator{
		result: &middleware.TokenValidationResult{
			UserID:   "user-abc",
			Role:     "user",
			Username: "alice",
		},
	}

	var capturedUserID, capturedRole string

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		capturedUserID = c.Request.Header.Get("X-User-ID")
		capturedRole = c.Request.Header.Get("X-User-Role")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "valid-token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "user-abc", capturedUserID)
	assert.Equal(t, "user", capturedRole)
}

func TestAuth_ValidToken_SetsAssumedByHeader(t *testing.T) {
	validator := &mockTokenValidator{
		result: &middleware.TokenValidationResult{
			UserID:    "target-user",
			Role:      "user",
			Username:  "bob",
			AssumedBy: "admin-123",
		},
	}

	var capturedAssumedBy string

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		capturedAssumedBy = c.Request.Header.Get("X-Assumed-By")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "assumed-token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "admin-123", capturedAssumedBy)
}

func TestAuth_ExpiredToken_Returns401(t *testing.T) {
	validator := &mockTokenValidator{
		err: fmt.Errorf("token expired"),
	}

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "expired-token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "UNAUTHORIZED")
}

func TestAuth_InvalidToken_Returns401(t *testing.T) {
	validator := &mockTokenValidator{
		err: fmt.Errorf("invalid token"),
	}

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "garbage"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "UNAUTHORIZED")
}

func TestAuth_MissingCookie_Returns401(t *testing.T) {
	validator := &mockTokenValidator{}

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "UNAUTHORIZED")
}

func TestAuth_UnauthenticatedRoutes_BypassValidation(t *testing.T) {
	validator := &mockTokenValidator{
		err: fmt.Errorf("should not be called"),
	}

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/register"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/refresh"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.Auth(validator, newSilentLogger()))
			router.Handle(tt.method, tt.path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

func TestAuth_UnauthenticatedRoute_WrongMethod_RequiresAuth(t *testing.T) {
	validator := &mockTokenValidator{
		err: fmt.Errorf("no token"),
	}

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/auth/register", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// GET /api/auth/register is NOT unauthenticated: only POST is.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/register", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuth_StripsIdentityHeaders_OnUnauthenticatedRoutes(t *testing.T) {
	validator := &mockTokenValidator{}

	var capturedUserID, capturedRole, capturedAssumedBy string

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.POST("/api/auth/register", func(c *gin.Context) {
		capturedUserID = c.Request.Header.Get("X-User-ID")
		capturedRole = c.Request.Header.Get("X-User-Role")
		capturedAssumedBy = c.Request.Header.Get("X-Assumed-By")
		c.Status(http.StatusOK)
	})

	// Client spoofs identity headers on an unauthenticated route.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	req.Header.Set("X-User-ID", "spoofed-user")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-Assumed-By", "spoofed-admin")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, capturedUserID, "X-User-ID should be stripped")
	assert.Empty(t, capturedRole, "X-User-Role should be stripped")
	assert.Empty(t, capturedAssumedBy, "X-Assumed-By should be stripped")
}

func TestAuth_StripsIdentityHeaders_BeforeValidation(t *testing.T) {
	validator := &mockTokenValidator{
		result: &middleware.TokenValidationResult{
			UserID:   "real-user",
			Role:     "user",
			Username: "alice",
		},
	}

	var capturedUserID, capturedRole string

	router := gin.New()
	router.Use(middleware.Auth(validator, newSilentLogger()))
	router.GET("/api/test", func(c *gin.Context) {
		capturedUserID = c.Request.Header.Get("X-User-ID")
		capturedRole = c.Request.Header.Get("X-User-Role")
		c.Status(http.StatusOK)
	})

	// Client spoofs headers, but auth succeeds with different identity.
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "valid-token"})
	req.Header.Set("X-User-ID", "spoofed-admin")
	req.Header.Set("X-User-Role", "admin")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "real-user", capturedUserID, "should use validated identity, not spoofed")
	assert.Equal(t, "user", capturedRole, "should use validated role, not spoofed")
}
