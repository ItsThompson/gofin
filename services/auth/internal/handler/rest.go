package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thompsnt/gofin/services/auth/internal/model"
	"github.com/thompsnt/gofin/services/auth/internal/service"
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
		slog.String("service", "auth"),
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
		slog.String("service", "auth"),
		slog.String("method", "POST /api/auth/login"),
		slog.String("user_id", user.ID),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

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

	// Refresh token: 7-day expiry, scoped to /api/auth/refresh
	c.SetCookie(
		"gofin_refresh",
		tokens.RefreshToken,
		int(7*24*time.Hour/time.Second),
		"/api/auth/refresh",
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
		slog.String("service", "auth"),
		slog.String("error", err.Error()),
	)
	c.JSON(http.StatusInternalServerError, model.ApiError{
		Code:    model.ErrInternalServerError,
		Message: "An unexpected error occurred",
	})
}
