package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	"github.com/ItsThompson/gofin/services/httpx"
	"github.com/ItsThompson/gofin/services/metrics"
)

// RESTHandler handles HTTP requests for the auth service.
type RESTHandler struct {
	authService     *service.AuthService
	logger          *slog.Logger
	cookieSecure    bool
	cookieDomain    string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewRESTHandler creates a new RESTHandler. accessTokenTTL and refreshTokenTTL
// set the auth-cookie max-ages; they are sourced from the same config as the
// JWT TTLs so the cookie lifetime always tracks the token lifetime.
func NewRESTHandler(authService *service.AuthService, logger *slog.Logger, cookieSecure bool, cookieDomain string, accessTokenTTL, refreshTokenTTL time.Duration) *RESTHandler {
	return &RESTHandler{
		authService:     authService,
		logger:          logger,
		cookieSecure:    cookieSecure,
		cookieDomain:    cookieDomain,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// RegisterRoutes registers every auth-owned route from the shared access
// Registry, binding each handler by ID. It is the single registration entry
// point shared by main.go and the registration coverage test, so a route can
// never be served without a Registry entry (which carries its access level).
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	access.BindRoutes("auth", h.handlers(), func(method, path string, handler gin.HandlerFunc) {
		r.Handle(method, path, handler)
	})
}

// handlers maps each auth Registry route ID to its gin handler. A Registry
// entry with no handler here (or a handler with no entry) is caught by
// BindRoutes at startup and by the registration coverage test.
func (h *RESTHandler) handlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"auth.register":            h.Register,
		"auth.login":               h.Login,
		"auth.refresh":             h.Refresh,
		"auth.logout":              h.Logout,
		"auth.me.get":              h.Me,
		"auth.me.update":           h.UpdateProfile,
		"auth.me.password":         h.ChangePassword,
		"auth.onboarding_complete": h.CompleteOnboarding,
		"auth.assume":              h.AssumeIdentity,
		"auth.restore":             h.RestoreIdentity,
		"admin.users.list":         h.ListUsers,
	}
}

// Register handles POST /api/auth/register.
func (h *RESTHandler) Register(c *gin.Context) {
	start := time.Now()

	var req model.RegisterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, tokens, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.setAuthCookies(c, tokens)

	h.logger.Info("register handler completed",
		slog.String("method", "POST /api/auth/register"),
		slog.String("user_id", user.ID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusCreated, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// Login handles POST /api/auth/login.
func (h *RESTHandler) Login(c *gin.Context) {
	start := time.Now()

	var req model.LoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, tokens, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.setAuthCookies(c, tokens)

	h.logger.Info("login handler completed",
		slog.String("method", "POST /api/auth/login"),
		slog.String("user_id", user.ID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// Me handles GET /api/auth/me.
// Returns the current user based on the X-User-ID header set by the gateway.
func (h *RESTHandler) Me(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// CompleteOnboarding handles POST /api/auth/onboarding-complete.
// Marks the user's onboarding as done and updates currency on the auth record.
func (h *RESTHandler) CompleteOnboarding(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CompleteOnboardingRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, err := h.authService.CompleteOnboarding(c.Request.Context(), userID, req.Currency)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// setAuthCookies sets the access and refresh token httpOnly cookies. Each
// cookie's max-age tracks the corresponding JWT TTL (sourced from config), so
// the cookie lifetime never drifts from the token lifetime.
func (h *RESTHandler) setAuthCookies(c *gin.Context, tokens *model.TokenPair) {
	c.SetSameSite(http.SameSiteStrictMode)

	// Access token cookie: root path so the Grafana auth proxy (on a separate
	// port) can also receive the cookie.
	c.SetCookie(
		"gofin_access",                        // name
		tokens.AccessToken,                    // value
		int(h.accessTokenTTL/time.Second),     // maxAge in seconds
		"/",                                   // path
		h.cookieDomain,                        // domain
		h.cookieSecure,                        // secure
		true,                                  // httpOnly
	)

	// Refresh token cookie: scoped to /api/auth (not /api/auth/refresh) so both
	// the refresh and logout endpoints can read the cookie.
	c.SetCookie(
		"gofin_refresh",
		tokens.RefreshToken,
		int(h.refreshTokenTTL/time.Second),
		"/api/auth",
		h.cookieDomain,
		h.cookieSecure,
		true,
	)
}

// handleError logs unexpected (non-apierr) errors before delegating to
// apierr.Respond, which owns the {code, message, fields?} wire mapping.
// apierr.Respond takes no logger, so logging here preserves the 500
// observability the service relies on.
func (h *RESTHandler) handleError(c *gin.Context, err error) {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		h.logger.Error("unexpected error",
			slog.String("error", err.Error()),
		)
	}
	apierr.Respond(c, err)
}

// clearAuthCookies removes both auth cookies by setting MaxAge to -1
// with the same path and flags as the originals.
func (h *RESTHandler) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("gofin_access", "", -1, "/", h.cookieDomain, h.cookieSecure, true)
	c.SetCookie("gofin_refresh", "", -1, "/api/auth", h.cookieDomain, h.cookieSecure, true)
}

// Refresh handles POST /api/auth/refresh.
// Reads the refresh token from the gofin_refresh cookie, validates it,
// blacklists the old token, and issues a new access + refresh pair.
func (h *RESTHandler) Refresh(c *gin.Context) {
	start := time.Now()

	cookie, err := c.Request.Cookie("gofin_refresh")
	if err != nil || cookie.Value == "" {
		apierr.Respond(c, apierr.Unauthorized("No refresh token provided"))
		return
	}

	user, tokens, err := h.authService.RefreshToken(c.Request.Context(), cookie.Value)
	if err != nil {
		metrics.TokenRefreshTotal.WithLabelValues("failure").Inc()
		h.handleError(c, err)
		return
	}

	metrics.TokenRefreshTotal.WithLabelValues("success").Inc()
	h.setAuthCookies(c, tokens)

	h.logger.Info("refresh handler completed",
		slog.String("method", "POST /api/auth/refresh"),
		slog.String("user_id", user.ID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// Logout handles POST /api/auth/logout.
// Blacklists the current refresh token and clears both cookies.
func (h *RESTHandler) Logout(c *gin.Context) {
	// Read the refresh cookie for blacklisting (best-effort)
	cookie, err := c.Request.Cookie("gofin_refresh")
	if err == nil && cookie.Value != "" {
		if logoutErr := h.authService.Logout(c.Request.Context(), cookie.Value); logoutErr != nil {
			h.logger.Error("failed to blacklist token during logout",
				slog.String("error", logoutErr.Error()),
			)
		}
	}

	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

// ListUsers handles GET /api/admin/users.
// Returns all registered users. The gateway enforces admin (operator) access
// via its centralized access-control middleware (GET /api/admin/users is
// classified Admin in the shared services/access registry).
func (h *RESTHandler) ListUsers(c *gin.Context) {
	start := time.Now()

	users, err := h.authService.ListUsers(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}

	adminUsers := make([]model.AdminUserResponse, 0, len(users))
	for _, user := range users {
		adminUsers = append(adminUsers, model.AdminUserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		})
	}

	h.logger.Info("list users handler completed",
		slog.String("method", "GET /api/admin/users"),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AdminUsersResponse{Users: adminUsers})
}

// AssumeIdentity handles POST /api/auth/assume.
// Generates a new JWT for the target user with the assumedBy claim.
// The gateway enforces admin (operator) access via its centralized
// access-control middleware (POST /api/auth/assume is classified Admin in the
// shared services/access registry).
func (h *RESTHandler) AssumeIdentity(c *gin.Context) {
	start := time.Now()

	adminUserID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.AssumeIdentityRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, tokens, err := h.authService.AssumeIdentity(c.Request.Context(), adminUserID, req.UserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.setAuthCookies(c, tokens)

	h.logger.Info("assume identity handler completed",
		slog.String("method", "POST /api/auth/assume"),
		slog.String("admin_user_id", adminUserID),
		slog.String("target_user_id", req.UserID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// RestoreIdentity handles POST /api/auth/restore.
// Reads the assumedBy claim from the current token (via X-Assumed-By header),
// generates fresh tokens for the original admin.
func (h *RESTHandler) RestoreIdentity(c *gin.Context) {
	start := time.Now()

	assumedBy := c.GetHeader("X-Assumed-By")
	if assumedBy == "" {
		apierr.Respond(c, apierr.Validation("No assumed identity to restore", nil))
		return
	}

	user, tokens, err := h.authService.RestoreIdentity(c.Request.Context(), assumedBy)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.setAuthCookies(c, tokens)

	h.logger.Info("restore identity handler completed",
		slog.String("method", "POST /api/auth/restore"),
		slog.String("admin_user_id", assumedBy),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// UpdateProfile handles PUT /api/auth/me.
// Updates the user's username, email, and currency.
func (h *RESTHandler) UpdateProfile(c *gin.Context) {
	start := time.Now()

	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.UpdateProfileRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Info("update profile handler completed",
		slog.String("method", "PUT /api/auth/me"),
		slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}

// ChangePassword handles POST /api/auth/me/password.
// Validates current password, updates to new password, revokes all tokens,
// and returns fresh cookies for the current session.
func (h *RESTHandler) ChangePassword(c *gin.Context) {
	start := time.Now()

	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.ChangePasswordRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user, tokens, err := h.authService.ChangePassword(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.setAuthCookies(c, tokens)

	h.logger.Info("change password handler completed",
		slog.String("method", "POST /api/auth/me/password"),
		slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	c.JSON(http.StatusOK, model.AuthResponse{
		User: user.ToResponse(),
	})
}
