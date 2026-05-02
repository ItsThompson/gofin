package handler

import (
	"log/slog"
	"net/http"
	"strconv"

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
		finance.PUT("/defaults", h.UpdateDefaults)
		finance.GET("/periods/current", h.GetCurrentPeriod)
		finance.POST("/periods", h.CreatePeriod)
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

// UpdateDefaults handles PUT /api/finance/defaults.
// Updates the user's default budget settings. Does not affect current or past periods.
func (h *RESTHandler) UpdateDefaults(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.UpdateDefaultsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
		return
	}

	defaults, err := h.financeService.UpdateDefaults(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// GetCurrentPeriod handles GET /api/finance/periods/current?year=YYYY&month=MM.
func (h *RESTHandler) GetCurrentPeriod(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "year and month query parameters are required",
		})
		return
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "year must be a valid integer",
		})
		return
	}

	month, err := strconv.ParseInt(monthStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "month must be a valid integer",
		})
		return
	}

	period, svcErr := h.financeService.GetCurrentPeriod(c.Request.Context(), userID, int32(year), int32(month))
	if svcErr != nil {
		h.handleError(c, svcErr)
		return
	}

	c.JSON(http.StatusOK, model.PeriodResponse{
		Period: period,
	})
}

// CreatePeriod handles POST /api/finance/periods.
func (h *RESTHandler) CreatePeriod(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.CreatePeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
		return
	}

	period, err := h.financeService.CreatePeriod(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, model.PeriodResponse{
		Period: period,
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
