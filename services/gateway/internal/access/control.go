package access

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/apierr"
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
// policy.
//
// It takes an injected resolve func rather than a policy value so the shared
// services/access module stays ignorant of gateway-native routes: the gateway
// composes GatewayResolve (health/metrics -> Public, else access.Resolve) and
// passes it in ("inject strategy, don't branch on context").
//
// Identity headers are stripped first so a client can never spoof them. Public
// and Deny routes short-circuit before any token read; every other level
// validates the gofin_access cookie, then applies the per-level role check.
//
// The per-level switch is fail-safe: only Authenticated passes without a role
// check, and any level that is not explicitly allowed is denied (403).
func AccessControl(validator TokenValidator, resolve func(method, path string) sharedaccess.Level, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		stripIdentityHeaders(c)

		level := resolve(c.Request.Method, c.Request.URL.Path)
		if level == sharedaccess.Public {
			c.Next()
			return
		}
		if level == sharedaccess.Deny {
			// An unclassified path is not a real route, so no identity is
			// needed: refuse it with a 403 before the cookie is read.
			abortForbidden(c, logger)
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
			// A bounded-timeout deadline means the auth dependency is unhealthy,
			// not that the client's token is invalid: fail fast with 503 so the
			// worker is freed, distinct from the 401 for a genuine rejection.
			if isValidationTimeout(err) {
				logger.Warn("auth validation timed out",
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("dependency", "auth"),
					slog.String("error", err.Error()),
				)
				abortUnavailable(c)
				return
			}
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
		case sharedaccess.Personal:
			if result.Role != roleUser {
				rejectForbidden(c, logger, result)
				return
			}
		case sharedaccess.Admin:
			if result.Role != roleAdmin {
				rejectForbidden(c, logger, result)
				return
			}
		case sharedaccess.Authenticated:
			// Any valid token passes; no role check.
		default:
			// Fail-safe by construction: Public and Deny are short-circuited
			// before token validation, so anything reaching here that is not
			// explicitly allowed (including an unrecognized future Access value)
			// is denied.
			rejectForbidden(c, logger, result)
			return
		}

		c.Next()
	}
}

// GatewayResolve is the resolver the gateway injects into AccessControl. It
// classifies the gateway-native endpoints (/health, /metrics, /readyz) as
// Public and delegates every /api route to the shared registry resolver.
// Keeping this composition in the gateway is why services/access never needs to
// know about gateway-owned routes.
func GatewayResolve(method, path string) sharedaccess.Level {
	if path == "/health" || path == "/metrics" || path == "/readyz" {
		return sharedaccess.Public
	}
	return sharedaccess.Resolve(method, path)
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

// abortUnauthorized ends the request with the 401 contract, encoded
// through the shared apierr wire struct. c.Abort halts the middleware chain
// (apierr.Respond only writes the body/status; it does not abort).
func abortUnauthorized(c *gin.Context, message string) {
	apierr.Respond(c, apierr.Unauthorized(message))
	c.Abort()
}

// isValidationTimeout reports whether a ValidateToken error is the bounded
// timeout tripping (a hung auth dependency) rather than a genuine auth
// rejection. The gateway bounds the RPC with context.WithTimeout in the
// concrete validator; a deadline surfaces either as a wrapped
// context.DeadlineExceeded or as a gRPC DeadlineExceeded status, so both prongs
// are checked. status.FromError unwraps the validator's fmt.Errorf %w wrap.
// Every other error (including all other gRPC codes) stays a 401.
func isValidationTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if st, ok := status.FromError(err); ok && st.Code() == codes.DeadlineExceeded {
		return true
	}
	return false
}

// abortUnavailable ends the request with 503 SERVICE_UNAVAILABLE, used when the
// auth dependency is unhealthy (validation timed out) rather than the client's
// token being invalid. SERVICE_UNAVAILABLE is a gateway-specific code (not one
// of apierr's shared codes), so the typed error is constructed inline.
func abortUnavailable(c *gin.Context) {
	apierr.Respond(c, &apierr.Error{
		Code:    "SERVICE_UNAVAILABLE",
		Message: "Authentication service unavailable",
		Status:  http.StatusServiceUnavailable,
	})
	c.Abort()
}

// rejectForbidden ends the request with the 403 FORBIDDEN / "Access denied"
// contract and emits a role-denied warn log.
func rejectForbidden(c *gin.Context, logger *slog.Logger, result *TokenValidationResult) {
	logger.Warn("access denied",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("role", result.Role),
		slog.String("user_id", result.UserID),
	)
	writeForbidden(c)
}

// abortForbidden ends the request with the same 403 FORBIDDEN/"Access denied"
// contract for a Deny (unclassified) route. No token was read, so there is no
// validated identity to log; only the method and path are recorded.
func abortForbidden(c *gin.Context, logger *slog.Logger) {
	logger.Warn("access denied for unclassified route",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
	)
	writeForbidden(c)
}

// writeForbidden emits the shared 403 body contract (FORBIDDEN / "Access
// denied") used by both the role-denied and unclassified-route paths, encoded
// through the shared apierr wire struct.
func writeForbidden(c *gin.Context) {
	apierr.Respond(c, &apierr.Error{
		Code:    apierr.CodeForbidden,
		Message: "Access denied",
		Status:  http.StatusForbidden,
	})
	c.Abort()
}
