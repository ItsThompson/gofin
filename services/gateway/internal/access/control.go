package access

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Identity headers the gateway sets for downstream services after a successful
// validation. They are stripped from every inbound request first so a client
// can never spoof them; the gateway is their only legitimate source.
const (
	headerUserID    = "X-User-ID"
	headerUserRole  = "X-User-Role"
	headerAssumedBy = "X-Assumed-By"
)

// accessCookie is the cookie carrying the access token validated on every
// non-public request.
const accessCookie = "gofin_access"

// Role values returned by the auth service in TokenValidationResult.Role. An
// assumed session carries roleUser, so it satisfies Personal routes.
const (
	roleUser  = "user"
	roleAdmin = "admin"
)

// AccessControl is the single gin middleware that enforces the gateway access
// policy, replacing the former Auth + RequireAdmin + AdminRouteGuard trio.
//
// For every request it:
//  1. strips the spoofable identity headers,
//  2. resolves the route's access level from the policy,
//  3. short-circuits Public routes with no token read,
//  4. otherwise validates the gofin_access cookie (401 on missing/invalid),
//  5. injects the validated identity as downstream headers, and
//  6. enforces the per-level role check (403 when the role is wrong).
func AccessControl(validator TokenValidator, policy Policy, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		stripIdentityHeaders(c)

		level := policy.resolve(c.Request.Method, c.Request.URL.Path)
		if level == Public {
			c.Next()
			return
		}

		cookie, err := c.Request.Cookie(accessCookie)
		if err != nil || cookie.Value == "" {
			logger.Warn("missing access token cookie",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
			)
			abortUnauthorized(c, "Authentication required")
			return
		}

		result, err := validator.ValidateToken(c.Request.Context(), cookie.Value)
		if err != nil {
			logger.Warn("token validation failed",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("error", err.Error()),
			)
			abortUnauthorized(c, "Invalid or expired token")
			return
		}

		setIdentityHeaders(c, result)

		switch level {
		case Personal:
			if result.Role != roleUser {
				rejectForbidden(c, logger, result)
				return
			}
		case Admin:
			if result.Role != roleAdmin {
				rejectForbidden(c, logger, result)
				return
			}
		case Public, Authenticated:
			// Public is short-circuited above; Authenticated needs only a valid
			// token, which we now have. Neither enforces a role.
		}

		c.Next()
	}
}

// stripIdentityHeaders removes client-supplied identity headers before
// resolution so they can never be spoofed. The gateway sets them only after a
// successful validation (see setIdentityHeaders).
func stripIdentityHeaders(c *gin.Context) {
	c.Request.Header.Del(headerUserID)
	c.Request.Header.Del(headerUserRole)
	c.Request.Header.Del(headerAssumedBy)
}

// setIdentityHeaders injects the validated identity for downstream services and
// stores the user id in the gin context for RequestLogger. X-Assumed-By is only
// forwarded when the session is assumed.
func setIdentityHeaders(c *gin.Context, result *TokenValidationResult) {
	c.Request.Header.Set(headerUserID, result.UserID)
	c.Request.Header.Set(headerUserRole, result.Role)
	c.Set(headerUserID, result.UserID)
	if result.AssumedBy != "" {
		c.Request.Header.Set(headerAssumedBy, result.AssumedBy)
	}
}

// abortUnauthorized ends the request with the unchanged 401 contract.
func abortUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    "UNAUTHORIZED",
		"message": message,
	})
}

// rejectForbidden ends the request with the unchanged 403 contract, preserving
// the role-denied warn log formerly emitted by middleware.rejectNonAdmin.
func rejectForbidden(c *gin.Context, logger *slog.Logger, result *TokenValidationResult) {
	logger.Warn("access denied",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("role", result.Role),
		slog.String("user_id", result.UserID),
	)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"code":    "FORBIDDEN",
		"message": "Access denied",
	})
}
