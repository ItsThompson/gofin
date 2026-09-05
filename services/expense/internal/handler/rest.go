package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	"github.com/ItsThompson/gofin/services/httpx"
)

// RESTHandler handles HTTP requests for the expense service.
type RESTHandler struct {
	expenseService *service.ExpenseService
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(expenseService *service.ExpenseService) *RESTHandler {
	return &RESTHandler{
		expenseService: expenseService,
	}
}

// RegisterRoutes registers every expense-owned route from the shared access
// Registry, binding each handler by ID. It is the single registration entry
// point shared by main.go and the registration coverage test, so a route can
// never be served without a Registry entry (which carries its access level).
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	access.BindRoutes("expense", h.handlers(), func(method, path string, handler gin.HandlerFunc) {
		r.Handle(method, path, handler)
	})
}

// handlers maps each expense Registry route ID to its gin handler. A Registry
// entry with no handler here (or a handler with no entry) is caught by
// BindRoutes at startup and by the registration coverage test.
func (h *RESTHandler) handlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"expense.create":        h.CreateExpense,
		"expense.list":          h.GetExpenses,
		"expense.suggestions":   h.GetExpenseSuggestions,
		"expense.prorata.group": h.GetExpensesInProRataGroup,
		"expense.get":           h.GetExpense,
		"expense.correct":       h.CorrectExpense,
		"expense.delete":       h.DeleteExpense,
		"expense.history":       h.GetCorrectionHistory,
	}
}

// CreateExpense handles POST /api/expenses.
func (h *RESTHandler) CreateExpense(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CreateExpenseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	expense, err := h.expenseService.CreateExpense(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, model.ExpenseResponse{
		Expense: expense,
	})
}

// GetExpenses handles GET /api/expenses?year=YYYY&month=MM&page=1&pageSize=50.
func (h *RESTHandler) GetExpenses(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		apierr.Respond(c, apierr.Validation("year and month query parameters are required", nil))
		return
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("year must be a valid integer", nil))
		return
	}

	month, err := strconv.ParseInt(monthStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("month must be a valid integer", nil))
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

	result, svcErr := h.expenseService.GetActiveExpensesForPeriod(c.Request.Context(), &model.GetExpensesRequest{
		UserID:   userID,
		Year:     int32(year),
		Month:    int32(month),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if svcErr != nil {
		h.respondError(c, svcErr)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetExpenseSuggestions handles GET /api/expenses/suggestions?page=1&pageSize=50.
func (h *RESTHandler) GetExpenseSuggestions(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	page, ok := parsePositiveIntQuery(c, "page", 1)
	if !ok {
		apierr.Respond(c, apierr.Validation("page must be a positive integer", nil))
		return
	}

	pageSize, ok := parsePositiveIntQuery(c, "pageSize", 50)
	if !ok || pageSize > 100 {
		apierr.Respond(c, apierr.Validation("pageSize must be a positive integer no greater than 100", nil))
		return
	}

	result, err := h.expenseService.GetExpenseSuggestions(c.Request.Context(), &model.ExpenseSuggestionRequest{
		UserID:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue int64) (int64, bool) {
	value := c.Query(key)
	if value == "" {
		return defaultValue, true
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 {
		return 0, false
	}
	return parsed, true
}

// GetExpense handles GET /api/expenses/:id.
func (h *RESTHandler) GetExpense(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	expense, err := h.expenseService.GetExpense(c.Request.Context(), userID, id)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.ExpenseResponse{
		Expense: expense,
	})
}

// respondError reports an unclassified failure and writes the shared error
// response. The operation comes from the route, so only the domain is per service.
func (h *RESTHandler) respondError(c *gin.Context, err error) {
	httpx.RespondError(c, err, errkit.Meta{
		Domain: "expenses",
		Msg:    "unexpected error",
	})
}

// CorrectExpense handles POST /api/expenses/:id/correct.
func (h *RESTHandler) CorrectExpense(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	expenseID := c.Param("id")

	var req model.CorrectExpenseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	expense, err := h.expenseService.CorrectExpense(c.Request.Context(), userID, expenseID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, model.ExpenseResponse{
		Expense: expense,
	})
}

// DeleteExpense handles DELETE /api/expenses/:id.
func (h *RESTHandler) DeleteExpense(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	expenseID := c.Param("id")
	if err := h.expenseService.DeleteExpense(c.Request.Context(), userID, expenseID); err != nil {
		h.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetCorrectionHistory handles GET /api/expenses/:id/history.
func (h *RESTHandler) GetCorrectionHistory(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	expenseID := c.Param("id")

	entries, err := h.expenseService.GetCorrectionHistory(c.Request.Context(), userID, expenseID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.CorrectionHistoryResponse{
		Entries: entries,
	})
}

// GetExpensesInProRataGroup handles GET /api/expenses/prorata/:groupId.
func (h *RESTHandler) GetExpensesInProRataGroup(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	groupID := c.Param("groupId")

	expenses, err := h.expenseService.GetExpensesInProRataGroup(c.Request.Context(), userID, groupID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.ProRataGroupResponse{
		Expenses: expenses,
	})
}
