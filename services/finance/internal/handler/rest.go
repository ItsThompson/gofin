package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

// RESTHandler handles HTTP requests for the finance service.
type RESTHandler struct {
	financeService *service.FinanceService
	logger         *slog.Logger
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(financeService *service.FinanceService, logger *slog.Logger) *RESTHandler {
	return &RESTHandler{
		financeService: financeService,
		logger:         logger,
	}
}

// RegisterRoutes sets up the Gin routes for finance endpoints.
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	finance := r.Group("/api/finance")
	{
		finance.POST("/onboarding", h.CompleteOnboarding)
		finance.GET("/defaults", h.GetDefaults)
	}
}

// CompleteOnboarding handles POST /api/finance/onboarding.
// Saves default settings and seeds default tags for the user.
func (h *RESTHandler) CompleteOnboarding(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.OnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
		return
	}

	defaults, err := h.financeService.CompleteOnboarding(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// GetDefaults handles GET /api/finance/defaults.
func (h *RESTHandler) GetDefaults(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	defaults, err := h.financeService.GetDefaults(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// handleError maps service errors to HTTP responses following the ApiError contract.
func (h *RESTHandler) handleError(c *gin.Context, err error) {
	if svcErr, ok := err.(*service.ServiceError); ok {
		c.JSON(svcErr.Status, model.ApiError{
			Code:    svcErr.Code,
			Message: svcErr.Message,
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
