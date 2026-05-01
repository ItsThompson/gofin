package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TokenValidationResult holds the identity returned by the auth service
// after a successful token validation.
type TokenValidationResult struct {
	UserID    string
	Role      string
	Username  string
	AssumedBy string
}

// TokenValidator abstracts the gRPC call to the auth service's ValidateToken RPC.
// This interface enables unit testing without a real gRPC connection.
type TokenValidator interface {
	ValidateToken(ctx context.Context, accessToken string) (*TokenValidationResult, error)
}

// unauthenticatedRoute defines a method+path pair that bypasses auth validation.
type unauthenticatedRoute struct {
	method string
	path   string
}

// unauthenticatedRoutes lists the routes that skip auth validation per the spec:
// registration, login, and token refresh.
var unauthenticatedRoutes = []unauthenticatedRoute{
	{method: http.MethodPost, path: "/api/auth/register"},
	{method: http.MethodPost, path: "/api/auth/login"},
	{method: http.MethodPost, path: "/api/auth/refresh"},
	{method: http.MethodGet, path: "/health"},
}

// isUnauthenticatedRoute checks whether a given method+path pair is exempt from auth.
func isUnauthenticatedRoute(method, path string) bool {
	for _, route := range unauthenticatedRoutes {
		if route.method == method && route.path == path {
			return true
		}
	}
	return false
}

// Auth returns Gin middleware that validates the access token on every request
// (except unauthenticated exceptions). On success, it injects X-User-ID and
// X-User-Role headers into the request before forwarding to downstream services.
func Auth(validator TokenValidator, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Strip identity headers on every request to prevent spoofing.
		// These are only set by the gateway after successful validation.
		c.Request.Header.Del("X-User-ID")
		c.Request.Header.Del("X-User-Role")
		c.Request.Header.Del("X-Assumed-By")

		if isUnauthenticatedRoute(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		cookie, err := c.Request.Cookie("gofin_access")
		if err != nil || cookie.Value == "" {
			logger.Warn("missing access token cookie",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Authentication required",
			})
			return
		}

		result, err := validator.ValidateToken(c.Request.Context(), cookie.Value)
		if err != nil {
			logger.Warn("token validation failed",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("error", err.Error()),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Invalid or expired token",
			})
			return
		}

		// Inject identity headers for downstream services.
		c.Request.Header.Set("X-User-ID", result.UserID)
		c.Request.Header.Set("X-User-Role", result.Role)

		// Store in Gin context so the logging middleware can read it.
		c.Set("X-User-ID", result.UserID)

		if result.AssumedBy != "" {
			c.Request.Header.Set("X-Assumed-By", result.AssumedBy)
		}

		c.Next()
	}
}
