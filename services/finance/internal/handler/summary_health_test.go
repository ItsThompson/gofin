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
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

// --- Dashboard Aggregation Handler Tests ---

func TestGetPeriodSummaryHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{ID: "e1", ReportingAmount: 50000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-01-05"},
			{ID: "e2", ReportingAmount: 20000, ExpenseType: "desires", TagID: "t2", ExpenseDate: "2025-01-10"},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.SummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-abc", resp.Summary.PeriodID)
	assert.Equal(t, int64(300000), resp.Summary.TotalBudget)
	assert.Equal(t, int64(70000), resp.Summary.TotalSpent)
	assert.Equal(t, int64(230000), resp.Summary.Remaining)
	assert.Equal(t, int64(150000), resp.Summary.Essentials.Allocated)
	assert.Equal(t, int64(50000), resp.Summary.Essentials.Spent)
	assert.Equal(t, int64(90000), resp.Summary.Desires.Allocated)
	assert.Equal(t, int64(20000), resp.Summary.Desires.Spent)
}

func TestGetPeriodSummaryHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(6)).
		Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=6", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
}

func TestGetPeriodSummaryHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", "/api/finance/summary"},
		{"missing month", "/api/finance/summary?year=2025"},
		{"missing year", "/api/finance/summary?month=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSONWithUserID(r, "GET", tt.query, "user-123", nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetPeriodSummaryHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=1", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Health Score Handler Tests ---

func TestGetHealthScoreHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	period := &model.BudgetPeriod{
		ID: "period-h", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}
	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).Return(period, nil)
	repo.On("GetDefaults", mock.Anything, "user-123").
		Return(&model.DefaultSettings{UserID: "user-123", Currency: "USD"}, nil)
	// Only the current period exists, so the stability window is empty.
	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{period}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]service.ExpenseData{
			{ID: "e1", ReportingAmount: 140000, ExpenseType: "essentials", ExpenseDate: "2026-05-05"},
			{ID: "e2", ReportingAmount: 80000, ExpenseType: "desires", ExpenseDate: "2026-05-06"},
			{ID: "e3", ReportingAmount: 40000, ExpenseType: "savings", ExpenseDate: "2026-05-07"},
		}, nil)

	// Fixed clock inside May 2026 -> the target month is provisional (computed
	// live, never persisted), so no stored read or upsert is expected.
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.HealthScore)
	assert.Equal(t, int32(2026), resp.HealthScore.Year)
	assert.Equal(t, int32(5), resp.HealthScore.Month)
	assert.Len(t, resp.HealthScore.Components, 3)
	assert.GreaterOrEqual(t, resp.HealthScore.Total, int32(0))
	assert.LessOrEqual(t, resp.HealthScore.Total, int32(100))
	assert.Equal(t, model.FormulaVersion, resp.HealthScore.FormulaVersion)
	assert.False(t, resp.HealthScore.ConfigureBudget)

	// total must equal the sum of components.
	var sum int32
	for _, component := range resp.HealthScore.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, resp.HealthScore.Total)
}

func TestGetHealthScoreHandler_ConfigureBudget(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "period-0", UserID: "user-123", Year: 2026, Month: 5, BudgetAmount: 0}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.HealthScore)
	assert.True(t, resp.HealthScore.ConfigureBudget)
	assert.Empty(t, resp.HealthScore.Components)
}

func TestGetHealthScoreHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(6)).Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=6", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
}

func TestGetHealthScoreHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHealthScoreTrendHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	periodFor := func(month int32) *model.BudgetPeriod {
		return &model.BudgetPeriod{
			ID: "p", UserID: "user-123", Year: 2026, Month: month,
			BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}
	}
	scalar := func(month, total int32, band string) *model.HealthScoreTrendPoint {
		return &model.HealthScoreTrendPoint{
			Year: 2026, Month: month, Total: total, Band: band,
			Provisional: false, FormulaVersion: model.FormulaVersion,
		}
	}

	repo.On("ListPeriods", mock.Anything, "user-123").
		Return([]*model.BudgetPeriod{periodFor(6), periodFor(5)}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-123").
		Return([]*model.HealthScoreTrendPoint{
			scalar(6, 82, model.HealthBandGreen),
			scalar(5, 70, model.HealthBandAmber),
		}, nil)

	// November 2026: both May and June are closed, so both come from the scalar read.
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026&month=6&months=6", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreTrendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Trends, 2)
	assert.Equal(t, int32(5), resp.Trends[0].Month, "points ascending")
	assert.Equal(t, int32(6), resp.Trends[1].Month)
	assert.Equal(t, int32(70), resp.Trends[0].Total)
	assert.Equal(t, int32(82), resp.Trends[1].Total)
	assert.Equal(t, model.FormulaVersion, resp.Trends[1].FormulaVersion)
}

func TestGetHealthScoreTrendHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHealthScoreTrendHandler_ClampsMonths(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	// months=99 is clamped, not rejected (capped at 12). With no periods the trend
	// is empty and the request still succeeds.
	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-123").Return([]*model.HealthScoreTrendPoint{}, nil)
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026&month=6&months=99", "user-123", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreTrendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Trends)
}

// --- GetHistoricalComparison Handler Tests ---

func TestGetHistoricalComparisonHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{
			ID: "p5", UserID: "user-123", Year: 2026, Month: 5,
			BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}, nil)

	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{
		{ID: "p5", UserID: "user-123", Year: 2026, Month: 5, BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
		{ID: "p4", UserID: "user-123", Year: 2026, Month: 4, BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
	}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]service.ExpenseData{{ReportingAmount: 80000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(4)).
		Return([]service.ExpenseData{{ReportingAmount: 60000}}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HistoricalComparisonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(80000), resp.Comparison.CurrentSpent)
	assert.Equal(t, int64(60000), resp.Comparison.PreviousSpent)
	assert.Nil(t, resp.Comparison.RollingAverage) // only 1 prior period
	assert.InDelta(t, 33.33, resp.Comparison.ChangePercent, 0.01)
}

func TestGetHistoricalComparisonHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistoricalComparisonHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
