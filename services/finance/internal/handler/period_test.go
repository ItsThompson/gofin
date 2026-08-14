package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// --- GetCurrentPeriod Handler Tests ---

func TestGetCurrentPeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      300000,
			ReportingCurrency: "USD",
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-abc", resp.Period.ID)
	assert.Equal(t, int32(2026), resp.Period.Year)
	assert.Equal(t, int32(5), resp.Period.Month)
	assert.Equal(t, int64(300000), resp.Period.BudgetAmount)
	assert.Equal(t, "USD", resp.Period.ReportingCurrency)
}

func TestGetCurrentPeriodHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
	assert.Contains(t, errResp.Message, "2026-05")
}

func TestGetCurrentPeriodHandler_MissingQueryParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", "/api/finance/periods/current"},
		{"missing month", "/api/finance/periods/current?year=2026"},
		{"missing year", "/api/finance/periods/current?month=5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSONWithUserID(r, "GET", tt.query, "user-123", nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetCurrentPeriodHandler_InvalidQueryParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=abc&month=5", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=abc", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCurrentPeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- CreatePeriod Handler Tests ---

func TestCreatePeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetLatestPeriod", mock.Anything, "user-123").Return(nil, nil)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(&model.BudgetPeriod{
			ID:                "period-new",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      300000,
			ReportingCurrency: "EUR",
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)
	repo.On("GetPendingProRata", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"reportingCurrency": "EUR",
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.CreatePeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-new", resp.Period.ID)
	assert.Equal(t, int32(2026), resp.Period.Year)
	assert.Equal(t, int32(5), resp.Period.Month)
	assert.Equal(t, int64(300000), resp.Period.BudgetAmount)
	assert.Equal(t, "EUR", resp.Period.ReportingCurrency)
}

func TestCreatePeriodHandler_ZeroBudget(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetLatestPeriod", mock.Anything, "user-123").Return(nil, nil)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(&model.BudgetPeriod{
			ID:                "period-zero",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      0,
			ReportingCurrency: "USD",
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)
	repo.On("GetPendingProRata", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      0,
		"reportingCurrency": "USD",
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.CreatePeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Period.BudgetAmount)
}

func TestCreatePeriodHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestCreatePeriodHandler_InvalidMonth(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             13,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePeriodHandler_UnsupportedReportingCurrency(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"reportingCurrency": "XYZ",
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrUnsupportedCurrency, errResp.Code)
	assert.Equal(t, "unsupported currency", errResp.Fields["reportingCurrency"])
	repo.AssertNotCalled(t, "CreatePeriod", mock.Anything, mock.Anything)
}

func TestCreatePeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- UpdatePeriod Handler Tests ---

func TestUpdatePeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	// Lock to May 2026
	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

	repo.On("GetPeriodByID", mock.Anything, "period-abc", "user-123").Return(&model.BudgetPeriod{
		ID: "period-abc", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}, nil)

	repo.On("UpdatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(&model.BudgetPeriod{
		ID: "period-abc", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 500000, EssentialsPercent: 60, DesiresPercent: 20, SavingsPercent: 20,
	}, nil)

	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 60,
		"desiresPercent":    20,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(500000), resp.Period.BudgetAmount)
	assert.Equal(t, int32(60), resp.Period.EssentialsPercent)
}

func TestUpdatePeriodHandler_PastPeriodLocked(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

	// Period is from April (past month)
	repo.On("GetPeriodByID", mock.Anything, "period-old", "user-123").Return(&model.BudgetPeriod{
		ID: "period-old", UserID: "user-123", Year: 2026, Month: 4,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}, nil)

	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-old", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusForbidden, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodLocked, errResp.Code)
}

func TestUpdatePeriodHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19, // sums to 99
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestUpdatePeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
