package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// --- Spending Trends Tests ---

func TestComputeSpendingTrends_NormalSixMonths(t *testing.T) {
	periods := make([]*model.BudgetPeriod, 6)
	expensesByMonth := make([][]ExpenseData, 6)
	years := []int32{2025, 2025, 2025, 2026, 2026, 2026}
	months := []int32{10, 11, 12, 1, 2, 3}

	for i := range periods {
		periods[i] = &model.BudgetPeriod{
			ID:                fmt.Sprintf("period-%d", i),
			BudgetAmount:      300000 + int64(i)*10000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}
		expensesByMonth[i] = []ExpenseData{
			{Amount: 100000, ReportingAmount: 100000, ExpenseType: "essentials"},
			{Amount: 50000, ReportingAmount: 50000, ExpenseType: "desires"},
			{Amount: 20000, ReportingAmount: 20000, ExpenseType: "savings"},
		}
	}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, months)

	assert.Len(t, result, 6)
	// First point is Oct 2025
	assert.Equal(t, int32(2025), result[0].Year)
	assert.Equal(t, int32(10), result[0].Month)
	// Last point is Mar 2026
	assert.Equal(t, int32(2026), result[5].Year)
	assert.Equal(t, int32(3), result[5].Month)

	// Verify aggregation for first month
	assert.Equal(t, int64(170000), result[0].TotalSpent)
	assert.Equal(t, int64(100000), result[0].EssentialsSpent)
	assert.Equal(t, int64(50000), result[0].DesiresSpent)
	assert.Equal(t, int64(20000), result[0].SavingsSpent)
	assert.Equal(t, int64(300000), result[0].BudgetAmount)

	// Budget percentages from period
	assert.Equal(t, float64(50), result[0].EssentialsPercent)
	assert.Equal(t, float64(30), result[0].DesiresPercent)
	assert.Equal(t, float64(20), result[0].SavingsPercent)
}

func TestComputeSpendingTrends_TwelveMonthsWithGaps(t *testing.T) {
	periods := make([]*model.BudgetPeriod, 12)
	expensesByMonth := make([][]ExpenseData, 12)
	years := make([]int32, 12)
	monthSlice := make([]int32, 12)

	// Only fill months 0, 1, 2, 5, 6 (gaps at 3, 4, 7-11)
	for i := range years {
		years[i] = 2025
		monthSlice[i] = int32(i + 1)
	}

	filledIndices := []int{0, 1, 2, 5, 6}
	for _, idx := range filledIndices {
		periods[idx] = &model.BudgetPeriod{
			BudgetAmount:      250000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}
		expensesByMonth[idx] = []ExpenseData{
			{Amount: 80000, ReportingAmount: 80000, ExpenseType: "essentials"},
		}
	}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, monthSlice)

	assert.Len(t, result, 12)

	// Filled months have data
	assert.Equal(t, int64(80000), result[0].TotalSpent)
	assert.Equal(t, int64(250000), result[0].BudgetAmount)

	// Gap months have zero values
	assert.Equal(t, int64(0), result[3].TotalSpent)
	assert.Equal(t, int64(0), result[3].BudgetAmount)
	assert.Equal(t, int64(0), result[3].EssentialsSpent)
}

func TestComputeSpendingTrends_ZeroExpenses(t *testing.T) {
	periods := []*model.BudgetPeriod{
		{BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
	}
	expensesByMonth := [][]ExpenseData{{}}
	years := []int32{2026}
	monthSlice := []int32{1}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, monthSlice)

	assert.Len(t, result, 1)
	assert.Equal(t, int64(0), result[0].TotalSpent)
	assert.Equal(t, int64(0), result[0].EssentialsSpent)
	assert.Equal(t, int64(0), result[0].DesiresSpent)
	assert.Equal(t, int64(0), result[0].SavingsSpent)
	assert.Equal(t, int64(300000), result[0].BudgetAmount)
	assert.Equal(t, float64(50), result[0].EssentialsPercent)
}

func TestComputeSpendingTrends_ZeroBudgetAmount(t *testing.T) {
	periods := []*model.BudgetPeriod{
		{BudgetAmount: 0, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
	}
	expensesByMonth := [][]ExpenseData{
		{{Amount: 5000, ReportingAmount: 5000, ExpenseType: "essentials"}},
	}
	years := []int32{2026}
	monthSlice := []int32{2}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, monthSlice)

	assert.Len(t, result, 1)
	assert.Equal(t, int64(5000), result[0].TotalSpent)
	assert.Equal(t, int64(0), result[0].BudgetAmount)
	// Budget percentages still come from the period
	assert.Equal(t, float64(50), result[0].EssentialsPercent)
}

func TestComputeSpendingTrends_SingleMonth(t *testing.T) {
	periods := []*model.BudgetPeriod{
		{BudgetAmount: 200000, EssentialsPercent: 60, DesiresPercent: 25, SavingsPercent: 15},
	}
	expensesByMonth := [][]ExpenseData{
		{
			{Amount: 40000, ReportingAmount: 40000, ExpenseType: "essentials"},
			{Amount: 30000, ReportingAmount: 30000, ExpenseType: "desires"},
			{Amount: 10000, ReportingAmount: 10000, ExpenseType: "savings"},
		},
	}
	years := []int32{2026}
	monthSlice := []int32{5}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, monthSlice)

	assert.Len(t, result, 1)
	assert.Equal(t, int32(2026), result[0].Year)
	assert.Equal(t, int32(5), result[0].Month)
	assert.Equal(t, int64(80000), result[0].TotalSpent)
	assert.Equal(t, int64(40000), result[0].EssentialsSpent)
	assert.Equal(t, int64(30000), result[0].DesiresSpent)
	assert.Equal(t, int64(10000), result[0].SavingsSpent)
	assert.Equal(t, float64(60), result[0].EssentialsPercent)
	assert.Equal(t, float64(25), result[0].DesiresPercent)
	assert.Equal(t, float64(15), result[0].SavingsPercent)
}

func TestComputeSpendingTrends_NilPeriodReturnsZeros(t *testing.T) {
	// When a period is nil (no budget created that month), all fields should be zero
	periods := []*model.BudgetPeriod{nil}
	expensesByMonth := [][]ExpenseData{nil}
	years := []int32{2026}
	monthSlice := []int32{3}

	result := ComputeSpendingTrends(periods, expensesByMonth, years, monthSlice)

	assert.Len(t, result, 1)
	assert.Equal(t, int32(2026), result[0].Year)
	assert.Equal(t, int32(3), result[0].Month)
	assert.Equal(t, int64(0), result[0].TotalSpent)
	assert.Equal(t, int64(0), result[0].BudgetAmount)
	assert.Equal(t, int64(0), result[0].EssentialsSpent)
	assert.Equal(t, int64(0), result[0].DesiresSpent)
	assert.Equal(t, int64(0), result[0].SavingsSpent)
	assert.Equal(t, float64(0), result[0].EssentialsPercent)
	assert.Equal(t, float64(0), result[0].DesiresPercent)
	assert.Equal(t, float64(0), result[0].SavingsPercent)
}

// --- GetSpendingTrends fan-out regression tests ---

// TestGetSpendingTrends_FanOutByteIdentical seeds a full 2026 window with two
// gaps (no period for April/September) and asserts the fan-out produces the
// expected result: chronological order preserved, per-month aggregation correct,
// gaps left as zero slots, exactly one read per non-nil period, and no read for
// the gaps.
func TestGetSpendingTrends_FanOutByteIdentical(t *testing.T) {
	exp := newCountingExpenseClient()
	var periods []*model.BudgetPeriod
	nonNil := 0
	for m := int32(12); m >= 1; m-- {
		if m == 4 || m == 9 {
			continue // gap: no BudgetPeriod for this month
		}
		nonNil++
		periods = append(periods, &model.BudgetPeriod{
			ID:                fmt.Sprintf("p-2026-%02d", m),
			UserID:            "user-1",
			Year:              2026,
			Month:             m,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		})
		exp.set(2026, m, []ExpenseData{
			{Amount: int64(m) * 1000, ReportingAmount: int64(m) * 1000, ExpenseType: "essentials"},
			{Amount: int64(m) * 500, ReportingAmount: int64(m) * 500, ExpenseType: "desires"},
		})
	}
	svc := newFanoutService(&fakeFanoutRepo{periods: periods}, exp)

	result, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, 12)
	require.NoError(t, err)
	require.Len(t, result, 12)

	for i := 0; i < 12; i++ {
		month := int32(i + 1)
		assert.Equal(t, int32(2026), result[i].Year)
		assert.Equal(t, month, result[i].Month, "results must stay in chronological (index) order")
		if month == 4 || month == 9 {
			assert.Equal(t, int64(0), result[i].TotalSpent)
			assert.Equal(t, int64(0), result[i].BudgetAmount)
			assert.Equal(t, 0, exp.counter.Count(monthOp(2026, month)), "gap month must issue no read")
			continue
		}
		assert.Equal(t, int64(month)*1500, result[i].TotalSpent)
		assert.Equal(t, int64(month)*1000, result[i].EssentialsSpent)
		assert.Equal(t, int64(month)*500, result[i].DesiresSpent)
		assert.Equal(t, int64(300000), result[i].BudgetAmount)
		assert.Equal(t, 1, exp.counter.Count(monthOp(2026, month)), "one read per non-nil period")
	}
	assert.Equal(t, nonNil, exp.counter.Total(), "total reads must equal the non-nil period count")
}

// TestGetSpendingTrends_FanOutRespectsLimit confirms the 12-wide window overlaps
// reads (not serial) while never exceeding SetLimit(dashboardFanoutLimit).
func TestGetSpendingTrends_FanOutRespectsLimit(t *testing.T) {
	exp := newCountingExpenseClient()
	exp.delay = 5 * time.Millisecond
	svc := newFanoutService(&fakeFanoutRepo{periods: seedYearPeriods(exp)}, exp)

	_, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, 12)
	require.NoError(t, err)

	assert.LessOrEqual(t, exp.maxConcurrent(), dashboardFanoutLimit,
		"in-flight reads must not exceed SetLimit(dashboardFanoutLimit)")
	assert.Greater(t, exp.maxConcurrent(), 1, "reads should overlap (fan-out), not run serially")
}

// TestGetSpendingTrends_FanOutWrapsError confirms a per-month read failure is
// surfaced with the period baked into the error (wrapped inside the goroutine).
func TestGetSpendingTrends_FanOutWrapsError(t *testing.T) {
	exp := newCountingExpenseClient()
	exp.failOn(2026, 7, errors.New("boom"))
	svc := newFanoutService(&fakeFanoutRepo{periods: seedYearPeriods(exp)}, exp)

	_, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, 12)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2026-07", "error must name the failing period")
	assert.Contains(t, err.Error(), "boom")
}

// TestGetSpendingTrends_FanOutCancelsSiblings proves the errgroup's first-error
// cancellation reaches sibling reads: when one period read fails fast, the
// derived gctx is cancelled, so reads not yet started return early (via the
// entry-time context check) instead of every period being read to completion.
// If the production goroutines passed the parent ctx (not gctx) to
// GetExpensesForPeriod, the siblings would not observe the cancellation and all
// twelve reads would complete, failing this assertion.
func TestGetSpendingTrends_FanOutCancelsSiblings(t *testing.T) {
	const window = 12
	exp := newCountingExpenseClient()
	exp.delay = 20 * time.Millisecond       // keep the first wave in flight past the failure
	exp.failOn(2026, 1, errors.New("boom")) // window[0]: fails fast, before its delay
	svc := newFanoutService(&fakeFanoutRepo{periods: seedYearPeriods(exp)}, exp)

	_, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, window)
	require.Error(t, err)
	assert.Less(t, exp.counter.Total(), window,
		"first-error must cancel sibling reads; not every period should be read to completion")
}
