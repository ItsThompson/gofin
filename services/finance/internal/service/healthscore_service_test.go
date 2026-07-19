package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

func TestGetHealthScore_ConfigureBudget(t *testing.T) {
	// Edge 1: budget_amount = 0 -> configure-budget response, no components.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "p0", UserID: "user-1", Year: 2026, Month: 5, BudgetAmount: 0}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.True(t, score.ConfigureBudget)
	assert.Empty(t, score.Components)
	assert.Equal(t, int32(2026), score.Year)
	assert.Equal(t, int32(5), score.Month)
	// Must not read defaults or expenses when there is no budget.
	repo.AssertNotCalled(t, "GetDefaults", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "GetExpensesForPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScore_Success_UsesDefaultsCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetDefaults", mock.Anything, "user-1").
		Return(&model.DefaultSettings{UserID: "user-1", Currency: "EUR"}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 130000),
			healthExpense("desires", 90000),
			healthExpense("savings", 30000),
		}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, int32(79), score.Total)
	assert.Equal(t, model.HealthKeySavings, score.Insight.Driver)
	// Currency from defaults drives the money symbol in the nudge.
	assert.Contains(t, score.Insight.Nudge, "€300")
}

func TestGetHealthScore_DefaultsCurrencyFallback(t *testing.T) {
	// VC3: nil defaults -> currency falls back to USD.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 130000),
			healthExpense("desires", 90000),
			healthExpense("savings", 30000),
		}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Contains(t, score.Insight.Nudge, "$300")
}

func TestGetHealthScore_PeriodNotFound(t *testing.T) {
	// Edge 2: no period -> PERIOD_NOT_FOUND (404).
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(6)).Return(nil, nil)

	_, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 6)

	apiErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodNotFound, apiErr.Code)
}
