package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
)

func TestRequireAdmin_AdminRole_Passes(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAdmin(newSilentLogger()))
	router.GET("/api/admin/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-User-ID", "admin-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestRequireAdmin_UserRole_Returns403(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAdmin(newSilentLogger()))
	router.GET("/api/admin/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("X-User-Role", "user")
	req.Header.Set("X-User-ID", "user-456")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "FORBIDDEN")
}

func TestRequireAdmin_MissingRole_Returns403(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAdmin(newSilentLogger()))
	router.GET("/api/admin/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "FORBIDDEN")
}

func TestRequireAdmin_EmptyRole_Returns403(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAdmin(newSilentLogger()))
	router.GET("/api/admin/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("X-User-Role", "")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAdminRouteGuard_AssumeEndpoint_AdminPasses(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AdminRouteGuard(newSilentLogger()))
	router.POST("/api/auth/assume", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/assume", nil)
	req.Header.Set("X-User-Role", "admin")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAdminRouteGuard_AssumeEndpoint_UserReturns403(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AdminRouteGuard(newSilentLogger()))
	router.POST("/api/auth/assume", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/assume", nil)
	req.Header.Set("X-User-Role", "user")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "FORBIDDEN")
}

func TestAdminRouteGuard_NonAdminRoute_Passes(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AdminRouteGuard(newSilentLogger()))
	router.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// POST /api/auth/login is not admin-only, so even 'user' role passes.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-User-Role", "user")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}
