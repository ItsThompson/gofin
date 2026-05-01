package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

// RESTHandler handles HTTP requests for the auth service.
type RESTHandler struct {
	authService  *service.AuthService
	logger       *slog.Logger
	cookieSecure bool
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(authService *service.AuthService, logger *slog.Logger, cookieSecure bool) *RESTHandler {
	return &RESTHandler{
		authService:  authService,
		logger:       logger,
		cookieSecure: cookieSecure,
	}
}

// RegisterRoutes sets up the Gin routes for auth endpoints.
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.Logout)
		auth.GET("/me", h.Me)
		auth.POST("/onboarding-complete", h.CompleteOnboarding)
	}
}

// Register handles POST /api/auth/register.
func (h *RESTHandler) Register(c *gin.Context) {
	start := time.Now()

	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
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
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
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
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.CompleteOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
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

// setAuthCookies sets the access and refresh token httpOnly cookies.
func (h *RESTHandler) setAuthCookies(c *gin.Context, tokens *model.TokenPair) {
	c.SetSameSite(http.SameSiteStrictMode)

	// Access token: 15-minute expiry, scoped to /api
	c.SetCookie(
		"gofin_access",          // name
		tokens.AccessToken,      // value
		int(15*time.Minute/time.Second), // maxAge in seconds
		"/api",                  // path
		"",                      // domain (empty = request host)
		h.cookieSecure,          // secure
		true,                    // httpOnly
	)

	// Refresh token: 7-day expiry, scoped to /api/auth
	// Path is /api/auth (not /api/auth/refresh) so both the refresh and
	// logout endpoints can read the cookie.
	c.SetCookie(
		"gofin_refresh",
		tokens.RefreshToken,
		int(7*24*time.Hour/time.Second),
		"/api/auth",
		"",
		h.cookieSecure,
		true,
	)
}

// handleError maps service errors to HTTP responses following the ApiError contract.
func (h *RESTHandler) handleError(c *gin.Context, err error) {
	if authErr, ok := err.(*service.AuthError); ok {
		c.JSON(authErr.Status, model.ApiError{
			Code:    authErr.Code,
			Message: authErr.Message,
		})
		return
	}

	h.logger.Error("unexpected error",
		slog.String("error", err.Error()),
	)
	c.JSON(http.StatusInternalServerError, model.ApiError{
		Code:    model.ErrInternalServerError,
		Message: "An unexpected error occurred",
	})
}

// clearAuthCookies removes both auth cookies by setting MaxAge to -1
// with the same path and flags as the originals.
func (h *RESTHandler) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("gofin_access", "", -1, "/api", "", h.cookieSecure, true)
	c.SetCookie("gofin_refresh", "", -1, "/api/auth", "", h.cookieSecure, true)
}

// Refresh handles POST /api/auth/refresh.
// Reads the refresh token from the gofin_refresh cookie, validates it,
// blacklists the old token, and issues a new access + refresh pair.
func (h *RESTHandler) Refresh(c *gin.Context) {
	start := time.Now()

	cookie, err := c.Request.Cookie("gofin_refresh")
	if err != nil || cookie.Value == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "No refresh token provided",
		})
		return
	}

	user, tokens, err := h.authService.RefreshToken(c.Request.Context(), cookie.Value)
	if err != nil {
		h.handleError(c, err)
		return
	}

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
