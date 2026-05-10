package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// adminOnlyRoutes lists method+path pairs outside /api/admin/* that require admin role.
// These are checked by AdminRouteGuard to enforce admin access on routes that
// live under other prefixes (e.g., /api/auth/assume).
var adminOnlyRoutes = []struct {
	method string
	path   string
}{
	{method: http.MethodPost, path: "/api/auth/assume"},
}

// adminOnlyPrefixes lists URL path prefixes that require admin role.
// Any request whose path starts with one of these prefixes is subject to
// admin enforcement (used for routes with dynamic segments like :id).
var adminOnlyPrefixes = []string{
	"/api/datarights/deletions",
}

// isAdminOnlyRoute checks whether a given method+path pair requires admin role.
// It checks both exact matches from adminOnlyRoutes and prefix matches from
// adminOnlyPrefixes.
func isAdminOnlyRoute(method, path string) bool {
	for _, route := range adminOnlyRoutes {
		if route.method == method && route.path == path {
			return true
		}
	}
	for _, prefix := range adminOnlyPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// rejectNonAdmin aborts the request with a 403 FORBIDDEN response.
func rejectNonAdmin(c *gin.Context, logger *slog.Logger) {
	logger.Warn("admin access denied",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("role", c.Request.Header.Get("X-User-Role")),
		slog.String("user_id", c.Request.Header.Get("X-User-ID")),
	)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"code":    "FORBIDDEN",
		"message": "Admin access required",
	})
}

// RequireAdmin returns Gin middleware that rejects requests where X-User-Role
// is not "admin". Apply to route groups like /api/admin/*.
// This middleware must run after Auth middleware, which sets the X-User-Role header.
func RequireAdmin(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.Request.Header.Get("X-User-Role")
		if role != "admin" {
			rejectNonAdmin(c, logger)
			return
		}

		c.Next()
	}
}

// AdminRouteGuard returns Gin middleware that enforces admin role on specific
// routes that live outside the /api/admin/* prefix. It checks the request
// against both the exact adminOnlyRoutes list (e.g., POST /api/auth/assume)
// and the adminOnlyPrefixes list (e.g., /api/datarights/deletions*).
func AdminRouteGuard(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAdminOnlyRoute(c.Request.Method, c.Request.URL.Path) {
			role := c.Request.Header.Get("X-User-Role")
			if role != "admin" {
				rejectNonAdmin(c, logger)
				return
			}
		}

		c.Next()
	}
}
