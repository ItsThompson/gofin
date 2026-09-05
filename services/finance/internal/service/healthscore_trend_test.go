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

// scalarPoint builds a stored, current-version scalar row (as the repository's
// batched read returns it: closed month, provisional false).
func scalarPoint(year, month, total int32) *model.HealthScoreTrendPoint {
	return &model.HealthScoreTrendPoint{
		Year: year, Month: month, Total: total, Band: model.Band(total),
		Provisional: false, FormulaVersion: model.FormulaVersion,
	}
}

func TestGetHealthScoreTrend_AllStoredAscending(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	totals := map[int32]int32{1: 50, 2: 55, 3: 60, 4: 65, 5: 70, 6: 75}
	periods := make([]*model.BudgetPeriod, 0, 6)
	scalars := make([]*model.HealthScoreTrendPoint, 0, 6)
	for month := int32(6); month >= 1; month-- { // DESC
		periods = append(periods, healthPeriodMonth(2026, month))
		scalars = append(scalars, scalarPoint(2026, month, totals[month]))
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return(scalars, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 6)
	for i, month := range []int32{1, 2, 3, 4, 5, 6} {
		assert.Equal(t, month, points[i].Month, "points ascending by month")
		assert.Equal(t, totals[month], points[i].Total)
		assert.False(t, points[i].Provisional)
	}
	// All stored at the current version: scalar read only, no JSONB read, no
	// compute, no upsert.
	repo.AssertNotCalled(t, "GetHealthScore", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "GetExpensesForPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScoreTrend_ProvisionalLast(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	nowJune := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJune })

	// Apr and May are stored scalars; June is the current provisional month with
	// no stored row, so it is computed live as the last point.
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{
		healthPeriodMonth(2026, 6), healthPeriodMonth(2026, 5), healthPeriodMonth(2026, 4),
	}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return([]*model.HealthScoreTrendPoint{
		scalarPoint(2026, 5, 65), scalarPoint(2026, 4, 60),
	}, nil)
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
	// The provisional month is computed but never stored, and stored scalars are
	// never re-read via the JSONB path.
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "GetHealthScore", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScoreTrend_SkipsGapMonths(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	// May is missing (no period), so it is skipped in the trend.
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{
		healthPeriodMonth(2026, 6), healthPeriodMonth(2026, 4), healthPeriodMonth(2026, 3),
	}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return([]*model.HealthScoreTrendPoint{
		scalarPoint(2026, 6, 75), scalarPoint(2026, 4, 65), scalarPoint(2026, 3, 60),
	}, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.Equal(t, int32(3), points[0].Month)
	assert.Equal(t, int32(4), points[1].Month)
	assert.Equal(t, int32(6), points[2].Month, "the missing May is skipped")
}

func TestGetHealthScoreTrend_StaleScalarRecomputes(t *testing.T) {
	// A stored scalar at an older formula version is not trusted: the month is
	// recomputed and upserted.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	staleMay := &model.HealthScoreTrendPoint{Year: 2026, Month: 5, Total: 70, Band: model.HealthBandAmber, FormulaVersion: 1}
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return([]*model.HealthScoreTrendPoint{staleMay}, nil)
	repo.On("GetHealthScore", mock.Anything, "user-1", int32(2026), int32(5)).Return(nil, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{healthExpense("desires", 80000)}, nil)
	repo.On("UpsertHealthScore", mock.Anything, "user-1", mock.Anything).Return(nil, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 5, 6)

	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, model.FormulaVersion, points[0].FormulaVersion, "stale scalar recomputed to current version")
	repo.AssertCalled(t, "UpsertHealthScore", mock.Anything, "user-1", mock.Anything)
}

// --- Health-score trend reporting currency ---

func TestGetHealthScoreTrend_ReportingCurrencyPerPoint(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowDecember })

	// Two stored months with different reporting currencies.
	usdPeriod := healthPeriodMonth(2026, 5)
	jpyPeriod := healthPeriodMonth(2026, 4)
	jpyPeriod.ReportingCurrencyCode = "JPY"
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{
		usdPeriod, jpyPeriod,
	}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return([]*model.HealthScoreTrendPoint{
		scalarPoint(2026, 5, 70), scalarPoint(2026, 4, 65),
	}, nil)

	points, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 6)

	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.Equal(t, "JPY", points[0].ReportingCurrencyCode, "first point gets period reporting currency")
	assert.Equal(t, "USD", points[1].ReportingCurrencyCode, "second point gets period reporting currency")
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
	scalars := make([]*model.HealthScoreTrendPoint, 0, len(ym))
	for _, p := range ym {
		periods = append(periods, healthPeriodMonth(p.year, p.month))
		scalars = append(scalars, scalarPoint(p.year, p.month, 60))
	}
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-1").Return(scalars, nil)

	def, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 0)
	require.NoError(t, err)
	assert.Len(t, def, 6, "months < 1 defaults to 6")

	big, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 6, 99)
	require.NoError(t, err)
	assert.Len(t, big, 8, "months > 12 is clamped; only the 8 available periods are returned")
}
