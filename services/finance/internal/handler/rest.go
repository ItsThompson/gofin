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
		finance.GET("/periods", h.ListPeriods)
		finance.POST("/periods", h.CreatePeriod)
		finance.GET("/tags", h.ListTags)

		// Dashboard aggregation endpoints
		finance.GET("/summary", h.GetPeriodSummary)
		finance.GET("/spending/by-tag", h.GetSpendingByTag)
		finance.GET("/spending/cumulative", h.GetCumulativeSpend)
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

// ListPeriods handles GET /api/finance/periods.
// Returns all budget periods for the authenticated user, ordered by year/month descending.
func (h *RESTHandler) ListPeriods(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	periods, err := h.financeService.ListPeriods(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.PeriodListResponse{
		Periods: periods,
	})
}

// ListTags handles GET /api/finance/tags.
// Returns all tags for the authenticated user, ordered alphabetically.
func (h *RESTHandler) ListTags(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	tags, err := h.financeService.ListTags(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TagListResponse{
		Tags: tags,
	})
}

// GetPeriodSummary handles GET /api/finance/summary?year=YYYY&month=MM.
func (h *RESTHandler) GetPeriodSummary(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	summary, err := h.financeService.GetPeriodSummary(c.Request.Context(), userID, year, month)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.SummaryResponse{
		Summary: summary,
	})
}

// GetSpendingByTag handles GET /api/finance/spending/by-tag?year=YYYY&month=MM.
func (h *RESTHandler) GetSpendingByTag(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	tags, err := h.financeService.GetSpendingByTag(c.Request.Context(), userID, year, month)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TagSpendingResponse{
		TagSpending: tags,
	})
}

// GetCumulativeSpend handles GET /api/finance/spending/cumulative?year=YYYY&month=MM.
func (h *RESTHandler) GetCumulativeSpend(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	points, err := h.financeService.GetCumulativeSpend(c.Request.Context(), userID, year, month)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.CumulativeSpendResponse{
		Points: points,
	})
}

// parseUserAndPeriodParams extracts and validates X-User-ID, year, and month from the request.
// Returns (userID, year, month, ok). When ok is false, an error response has already been sent.
func (h *RESTHandler) parseUserAndPeriodParams(c *gin.Context) (string, int32, int32, bool) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return "", 0, 0, false
	}

	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "year and month query parameters are required",
		})
		return "", 0, 0, false
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "year must be a valid integer",
		})
		return "", 0, 0, false
	}

	month, err := strconv.ParseInt(monthStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "month must be a valid integer",
		})
		return "", 0, 0, false
	}

	return userID, int32(year), int32(month), true
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
