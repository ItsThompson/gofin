package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

var nowDecember = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

// mockStoredMonth registers a stored, current-version score for (year, month)
// and returns the matching period for the ListPeriods window.
func mockStoredMonth(repo *mockRepo, year, month, total int32) *model.BudgetPeriod {
	repo.On("GetHealthScore", mock.Anything, "user-1", year, month).Return(&model.HealthScore{
		Year: year, Month: month, Total: total, Band: model.Band(total),
		Provisional: false, FormulaVersion: model.FormulaVersion,
		Components: []model.HealthComponent{{Key: model.HealthKeyBudget, Score: total, Max: 100}},
	}, nil)
	return healthPeriodMonth(year, month)
}

func TestGetHealthScoreTrend_AllStoredAscending(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	totals := map[int32]int32{1: 50, 2: 55, 3: 60, 4: 65, 5: 70, 6: 75}
	periods := make([]*model.BudgetPeriod, 0, 6)
	for month := int32(6); month >= 1; month-- { // DESC
		periods = append(periods, mockStoredMonth(repo, 2026, month, totals[month]))
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 6)
	for i, month := range []int32{1, 2, 3, 4, 5, 6} {
		assert.Equal(t, month, points[i].Month, "points ascending by month")
		assert.Equal(t, totals[month], points[i].Total)
		assert.False(t, points[i].Provisional)
	}
	// All stored at the current version: no recompute, no expense read, no upsert.
	expClient.AssertNotCalled(t, "GetExpensesForPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScoreTrend_ProvisionalLast(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	nowJune := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJune })

	aprP := mockStoredMonth(repo, 2026, 4, 60)
	mayP := mockStoredMonth(repo, 2026, 5, 65)
	junP := healthPeriodMonth(2026, 6) // current month, computed live
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{junP, mayP, aprP}, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(&model.DefaultSettings{Currency: "USD"}, nil)
	for month := int32(4); month <= 6; month++ {
		expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), month).
			Return([]ExpenseData{healthExpense("desires", 80000)}, nil)
	}

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.Equal(t, int32(4), points[0].Month)
	assert.Equal(t, int32(5), points[1].Month)
	assert.Equal(t, int32(6), points[2].Month)
	assert.False(t, points[0].Provisional)
	assert.False(t, points[1].Provisional)
	assert.True(t, points[2].Provisional, "the current month is the last point and provisional")
	// The provisional month is computed but never stored.
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "GetHealthScore", mock.Anything, "user-1", int32(2026), int32(6))
}

func TestGetHealthScoreTrend_SkipsGapMonths(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	// May is missing (no period), so it is skipped in the trend.
	junP := mockStoredMonth(repo, 2026, 6, 75)
	aprP := mockStoredMonth(repo, 2026, 4, 65)
	marP := mockStoredMonth(repo, 2026, 3, 60)
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{junP, aprP, marP}, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.Equal(t, int32(3), points[0].Month)
	assert.Equal(t, int32(4), points[1].Month)
	assert.Equal(t, int32(6), points[2].Month, "the missing May is skipped")
}

func TestGetHealthScoreTrend_ClampsMonths(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	// Eight consecutive stored months (2025-11 .. 2026-06), DESC.
	ym := []struct{ year, month int32 }{
		{2026, 6}, {2026, 5}, {2026, 4}, {2026, 3}, {2026, 2}, {2026, 1}, {2025, 12}, {2025, 11},
	}
	periods := make([]*model.BudgetPeriod, 0, len(ym))
	for _, p := range ym {
		periods = append(periods, mockStoredMonth(repo, p.year, p.month, 60))
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)

	def, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 0)
	require.NoError(t, err)
	assert.Len(t, def, 6, "months < 1 defaults to 6")

	big, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 99)
	require.NoError(t, err)
	assert.Len(t, big, 8, "months > 12 is clamped; only the 8 available periods are returned")
}
