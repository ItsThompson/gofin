package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	"github.com/ItsThompson/gofin/services/httpx"
	currencycatalog "github.com/ItsThompson/gofin/services/shared/currency"
)

// RESTHandler handles HTTP requests for the finance service.
type RESTHandler struct {
	financeService *service.FinanceService
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(financeService *service.FinanceService) *RESTHandler {
	return &RESTHandler{
		financeService: financeService,
	}
}

// RegisterRoutes registers every finance-owned route from the shared access
// Registry, binding each handler by ID. It is the single registration entry
// point shared by main.go and the registration coverage test, so a route can
// never be served without a Registry entry (which carries its access level).
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	access.BindRoutes("finance", h.handlers(), func(method, path string, handler gin.HandlerFunc) {
		r.Handle(method, path, handler)
	})
}

// handlers maps each finance Registry route ID to its gin handler. A Registry
// entry with no handler here (or a handler with no entry) is caught by
// BindRoutes at startup and by the registration coverage test.
func (h *RESTHandler) handlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"finance.onboarding":          h.CompleteOnboarding,
		"finance.defaults.get":        h.GetDefaults,
		"finance.defaults.update":     h.UpdateDefaults,
		"finance.currencies.list":     h.ListCurrencies,
		"finance.periods.current":     h.GetCurrentPeriod,
		"finance.periods.list":        h.ListPeriods,
		"finance.periods.create":      h.CreatePeriod,
		"finance.periods.update":      h.UpdatePeriod,
		"finance.tags.list":           h.ListTags,
		"finance.tags.create":         h.CreateTag,
		"finance.tags.update":         h.UpdateTag,
		"finance.tags.delete":         h.DeleteTag,
		"finance.summary":             h.GetPeriodSummary,
		"finance.spending.by_tag":     h.GetSpendingByTag,
		"finance.spending.cumulative": h.GetCumulativeSpend,
		"finance.spending.comparison": h.GetHistoricalComparison,
		"finance.spending.trends":     h.GetSpendingTrends,
		"finance.prorata.create":      h.CreateProRataExpense,
		"finance.prorata.upcoming":    h.GetUpcomingProRata,
		"finance.health_score":        h.GetHealthScore,
		"finance.health_score.trend":  h.GetHealthScoreTrend,
	}
}

// CompleteOnboarding handles POST /api/finance/onboarding.
// Saves default settings and seeds default tags for the user.
func (h *RESTHandler) CompleteOnboarding(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.OnboardingRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	defaults, err := h.financeService.CompleteOnboarding(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// GetDefaults handles GET /api/finance/defaults.
func (h *RESTHandler) GetDefaults(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	defaults, err := h.financeService.GetDefaults(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// UpdateDefaults handles PUT /api/finance/defaults.
// Updates the user's default budget settings. Does not affect current or past periods.
func (h *RESTHandler) UpdateDefaults(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.UpdateDefaultsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	defaults, err := h.financeService.UpdateDefaults(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DefaultsResponse{
		Defaults: defaults,
	})
}

// ListCurrencies handles GET /api/finance/currencies.
// The catalog is static reference data owned by the shared currency package,
// so the handler reads it directly instead of routing through the service layer.
func (h *RESTHandler) ListCurrencies(c *gin.Context) {
	_, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	definitions := currencycatalog.All()
	currencies := make([]model.CurrencyData, len(definitions))
	for i, definition := range definitions {
		currencies[i] = model.CurrencyData{
			Code:            definition.Code,
			Symbol:          definition.Symbol,
			Name:            definition.Name,
			MinorUnitDigits: definition.MinorUnitDigits,
		}
	}

	c.JSON(http.StatusOK, model.CurrencyListResponse{
		Currencies: currencies,
	})
}

// GetCurrentPeriod handles GET /api/finance/periods/current?year=YYYY&month=MM.
func (h *RESTHandler) GetCurrentPeriod(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		apierr.Respond(c, apierr.Validation("year and month query parameters are required", map[string]string{
			"year":  "required",
			"month": "required",
		}))
		return
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("year must be a valid integer", map[string]string{"year": "must be a valid integer"}))
		return
	}

	month, err := strconv.ParseInt(monthStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("month must be a valid integer", map[string]string{"month": "must be a valid integer"}))
		return
	}

	period, svcErr := h.financeService.GetCurrentPeriod(c.Request.Context(), userID, int32(year), int32(month))
	if svcErr != nil {
		h.respondError(c, svcErr)
		return
	}

	c.JSON(http.StatusOK, model.PeriodResponse{
		Period: period,
	})
}

// CreatePeriod handles POST /api/finance/periods.
// Creates a budget period, auto-creates missed months, and applies pending pro-rata.
func (h *RESTHandler) CreatePeriod(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CreatePeriodRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	result, err := h.financeService.CreatePeriodWithProRata(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// ListPeriods handles GET /api/finance/periods.
// Returns all budget periods for the authenticated user, ordered by year/month descending.
func (h *RESTHandler) ListPeriods(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	periods, err := h.financeService.ListPeriods(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.PeriodListResponse{
		Periods: periods,
	})
}

// ListTags handles GET /api/finance/tags.
// Returns all tags for the authenticated user, ordered alphabetically.
func (h *RESTHandler) ListTags(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	tags, err := h.financeService.ListTags(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TagListResponse{
		Tags: tags,
	})
}

// CreateTag handles POST /api/finance/tags.
// Creates a new custom tag. Name must be unique per user (case-insensitive), max 50 chars.
func (h *RESTHandler) CreateTag(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CreateTagRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	tag, err := h.financeService.CreateTag(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, model.TagResponse{
		Tag: tag,
	})
}

// UpdateTag handles PUT /api/finance/tags/:id.
// Renames a tag (any tag, including defaults).
func (h *RESTHandler) UpdateTag(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	tagID := c.Param("id")

	var req model.UpdateTagRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	tag, err := h.financeService.UpdateTag(c.Request.Context(), userID, tagID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TagResponse{
		Tag: tag,
	})
}

// DeleteTag handles DELETE /api/finance/tags/:id.
// Deletes a tag only if it's not a default and not referenced by expenses or schedules.
func (h *RESTHandler) DeleteTag(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	tagID := c.Param("id")

	err := h.financeService.DeleteTag(c.Request.Context(), userID, tagID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPeriodSummary handles GET /api/finance/summary?year=YYYY&month=MM.
func (h *RESTHandler) GetPeriodSummary(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	summary, err := h.financeService.GetPeriodSummary(c.Request.Context(), userID, year, month)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.SummaryResponse{
		Summary: summary,
	})
}

// GetHealthScore handles GET /api/finance/health-score?year=YYYY&month=MM.
func (h *RESTHandler) GetHealthScore(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	score, err := h.financeService.GetHealthScore(c.Request.Context(), userID, year, month)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.HealthScoreResponse{
		HealthScore: score,
	})
}

// GetHealthScoreTrend handles GET /api/finance/health-score/trend?year=YYYY&month=MM&months=6|12.
func (h *RESTHandler) GetHealthScoreTrend(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	// months follows the clamp policy (default 6, cap 12): a non-numeric value
	// defaults to 6 and the service clamps the range, so the handler, service, and
	// mock all agree.
	months, err := strconv.ParseInt(c.DefaultQuery("months", "6"), 10, 32)
	if err != nil {
		months = 6
	}

	trends, err := h.financeService.GetHealthScoreTrend(c.Request.Context(), userID, year, month, int32(months))
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.HealthScoreTrendResponse{
		Trends: trends,
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
		h.respondError(c, err)
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
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.CumulativeSpendResponse{
		Points: points,
	})
}

// UpdatePeriod handles PUT /api/finance/periods/:id.
// Updates the current period's budget and E/D/S split. Past periods return 403.
func (h *RESTHandler) UpdatePeriod(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	periodID := c.Param("id")
	if rejectImmutableReportingCurrencyUpdate(c) {
		return
	}

	var req model.UpdatePeriodRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	period, err := h.financeService.UpdatePeriod(c.Request.Context(), userID, periodID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.PeriodResponse{
		Period: period,
	})
}

func rejectImmutableReportingCurrencyUpdate(c *gin.Context) bool {
	fields := immutableReportingCurrencyFields(c)
	if len(fields) == 0 {
		return false
	}

	apierr.Respond(c, &apierr.Error{
		Code:    model.ErrReportingCurrencyImmutable,
		Message: "Reporting currency cannot be changed after period creation",
		Status:  http.StatusBadRequest,
		Fields:  fields,
	})
	return true
}

func immutableReportingCurrencyFields(c *gin.Context) map[string]string {
	body, err := c.GetRawData()
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	fields := map[string]string{}
	for _, field := range []string{"reportingCurrency", "reporting_currency"} {
		if _, ok := payload[field]; ok {
			fields[field] = "immutable"
		}
	}
	return fields
}

// GetHistoricalComparison handles GET /api/finance/spending/comparison?year=YYYY&month=MM.
func (h *RESTHandler) GetHistoricalComparison(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	comparison, err := h.financeService.GetHistoricalComparison(c.Request.Context(), userID, year, month)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.HistoricalComparisonResponse{
		Comparison: comparison,
	})
}

// GetSpendingTrends handles GET /api/finance/spending/trends?year=YYYY&month=MM&months=6|12.
func (h *RESTHandler) GetSpendingTrends(c *gin.Context) {
	userID, year, month, ok := h.parseUserAndPeriodParams(c)
	if !ok {
		return
	}

	monthsStr := c.DefaultQuery("months", "6")
	months, err := strconv.ParseInt(monthsStr, 10, 32)
	if err != nil || months < 1 || months > 12 {
		apierr.Respond(c, apierr.Validation("months must be between 1 and 12", map[string]string{"months": "must be between 1 and 12"}))
		return
	}

	trends, err := h.financeService.GetSpendingTrends(c.Request.Context(), userID, year, month, int32(months))
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.TrendResponse{
		Trends: trends,
	})
}

// parseUserAndPeriodParams extracts and validates X-User-ID, year, and month from the request.
// Returns (userID, year, month, ok). When ok is false, an error response has already been sent.
func (h *RESTHandler) parseUserAndPeriodParams(c *gin.Context) (string, int32, int32, bool) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return "", 0, 0, false
	}

	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		apierr.Respond(c, apierr.Validation("year and month query parameters are required", map[string]string{
			"year":  "required",
			"month": "required",
		}))
		return "", 0, 0, false
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("year must be a valid integer", map[string]string{"year": "must be a valid integer"}))
		return "", 0, 0, false
	}

	month, err := strconv.ParseInt(monthStr, 10, 32)
	if err != nil {
		apierr.Respond(c, apierr.Validation("month must be a valid integer", map[string]string{"month": "must be a valid integer"}))
		return "", 0, 0, false
	}

	return userID, int32(year), int32(month), true
}

// CreateProRataExpense handles POST /api/finance/prorata.
// Creates a pro-rata expense: writes first installment, schedules future ones.
func (h *RESTHandler) CreateProRataExpense(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CreateProRataRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	result, err := h.financeService.CreateProRataExpense(c.Request.Context(), userID, &req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetUpcomingProRata handles GET /api/finance/prorata/upcoming.
// Returns all pending pro-rata schedules for the user.
func (h *RESTHandler) GetUpcomingProRata(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	schedules, err := h.financeService.GetUpcomingProRata(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.UpcomingProRataResponse{
		Schedules: schedules,
	})
}

// respondError reports an unclassified failure and writes the shared error
// response. The operation comes from the route, so only the domain is per service.
func (h *RESTHandler) respondError(c *gin.Context, err error) {
	httpx.RespondError(c, err, errkit.Meta{
		Domain: "budgets",
		Msg:    "unexpected error",
	})
}
