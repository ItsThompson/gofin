package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
)

// RESTHandler handles HTTP requests for the expense service.
type RESTHandler struct {
	expenseService *service.ExpenseService
	logger         *slog.Logger
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(expenseService *service.ExpenseService, logger *slog.Logger) *RESTHandler {
	return &RESTHandler{
		expenseService: expenseService,
		logger:         logger,
	}
}

// RegisterRoutes sets up the Gin routes for expense endpoints.
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	expenses := r.Group("/api/expenses")
	{
		expenses.POST("", h.CreateExpense)
		expenses.GET("", h.GetExpenses)
		expenses.GET("/:id", h.GetExpense)
	}
}

// CreateExpense handles POST /api/expenses.
func (h *RESTHandler) CreateExpense(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body",
		})
		return
	}

	expense, err := h.expenseService.CreateExpense(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, model.ExpenseResponse{
		Expense: expense,
	})
}

// GetExpenses handles GET /api/expenses?year=YYYY&month=MM&page=1&pageSize=50.
func (h *RESTHandler) GetExpenses(c *gin.Context) {
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

	page := int64(1)
	if pageStr := c.Query("page"); pageStr != "" {
		parsed, parseErr := strconv.ParseInt(pageStr, 10, 32)
		if parseErr == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := int64(50)
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		parsed, parseErr := strconv.ParseInt(pageSizeStr, 10, 32)
		if parseErr == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	result, svcErr := h.expenseService.GetExpensesForPeriod(c.Request.Context(), &model.GetExpensesRequest{
		UserID:   userID,
		Year:     int32(year),
		Month:    int32(month),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if svcErr != nil {
		h.handleError(c, svcErr)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetExpense handles GET /api/expenses/:id.
func (h *RESTHandler) GetExpense(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	id := c.Param("id")
	expense, err := h.expenseService.GetExpense(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.ExpenseResponse{
		Expense: expense,
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
