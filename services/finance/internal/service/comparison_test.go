package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// --- Historical Comparison Tests ---

func TestGetHistoricalComparison_WithThreePriorPeriods(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// Current period: May 2026
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(makePeriod("p5", 2026, 5), nil)

	// List all periods (DESC order)
	periods := []*model.BudgetPeriod{
		makePeriod("p5", 2026, 5),
		makePeriod("p4", 2026, 4),
		makePeriod("p3", 2026, 3),
		makePeriod("p2", 2026, 2),
		makePeriod("p1", 2026, 1),
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	// Expense totals: current=80000, prev=70000, p3=60000, p2=50000
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{{Amount: 80000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(4)).
		Return([]ExpenseData{{Amount: 70000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(3)).
		Return([]ExpenseData{{Amount: 60000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return([]ExpenseData{{Amount: 50000}}, nil)

	result, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, int64(80000), result.CurrentSpent)
	assert.Equal(t, int64(70000), result.PreviousSpent)
	require.NotNil(t, result.RollingAverage)
	// Rolling avg of 3 prior: (70000 + 60000 + 50000) / 3 = 60000
	assert.Equal(t, int64(60000), *result.RollingAverage)
	// Change: (80000 - 70000) / 70000 = 14.29%
	assert.InDelta(t, 14.29, result.ChangePercent, 0.01)
}

func TestGetHistoricalComparison_WithTwoPriorPeriods(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(3)).
		Return(makePeriod("p3", 2026, 3), nil)

	periods := []*model.BudgetPeriod{
		makePeriod("p3", 2026, 3),
		makePeriod("p2", 2026, 2),
		makePeriod("p1", 2026, 1),
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(3)).
		Return([]ExpenseData{{Amount: 50000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return([]ExpenseData{{Amount: 40000}}, nil)

	result, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 3)

	require.NoError(t, err)
	assert.Equal(t, int64(50000), result.CurrentSpent)
	assert.Equal(t, int64(40000), result.PreviousSpent)
	// Only 2 prior periods: rolling average should be null
	assert.Nil(t, result.RollingAverage)
	// Change: (50000 - 40000) / 40000 = 25%
	assert.InDelta(t, 25.0, result.ChangePercent, 0.01)
}

func TestGetHistoricalComparison_OnlyOnePeriod(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(1)).
		Return(makePeriod("p1", 2026, 1), nil)

	periods := []*model.BudgetPeriod{
		makePeriod("p1", 2026, 1),
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(1)).
		Return([]ExpenseData{{Amount: 30000}}, nil)

	result, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(30000), result.CurrentSpent)
	assert.Equal(t, int64(0), result.PreviousSpent)
	assert.Nil(t, result.RollingAverage)
	assert.InDelta(t, 0.0, result.ChangePercent, 0.01)
}

func TestGetHistoricalComparison_PreviousZeroSpentCurrentPositive(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return(makePeriod("p2", 2026, 2), nil)

	periods := []*model.BudgetPeriod{
		makePeriod("p2", 2026, 2),
		makePeriod("p1", 2026, 1),
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return([]ExpenseData{{Amount: 50000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(1)).
		Return([]ExpenseData{}, nil) // zero spent

	result, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 2)

	require.NoError(t, err)
	assert.Equal(t, int64(50000), result.CurrentSpent)
	assert.Equal(t, int64(0), result.PreviousSpent)
	// When previous is 0 but current is positive, changePercent = 100%
	assert.InDelta(t, 100.0, result.ChangePercent, 0.01)
}

func TestGetHistoricalComparison_DecreaseFromPrevious(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return(makePeriod("p2", 2026, 2), nil)

	periods := []*model.BudgetPeriod{
		makePeriod("p2", 2026, 2),
		{ID: "p1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 300000,
			EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
			CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(2)).
		Return([]ExpenseData{{Amount: 30000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(1)).
		Return([]ExpenseData{{Amount: 60000}}, nil)

	result, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 2)

	require.NoError(t, err)
	assert.Equal(t, int64(30000), result.CurrentSpent)
	assert.Equal(t, int64(60000), result.PreviousSpent)
	// Change: (30000 - 60000) / 60000 = -50%
	assert.InDelta(t, -50.0, result.ChangePercent, 0.01)
}
