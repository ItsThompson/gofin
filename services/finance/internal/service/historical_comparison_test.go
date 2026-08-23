package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// --- Cross-currency historical comparison ---

// TestGetHistoricalComparison_MixedCurrencyNotComparable asserts that when the
// current and previous periods have different reporting currencies, the
// Comparable flag is false, ChangePercent is zeroed, and the rolling average is
// suppressed.
func TestGetHistoricalComparison_MixedCurrencyNotComparable(t *testing.T) {
	exp := newCountingExpenseClient()
	currentPeriod := makePeriod("p12", 2026, 12)
	currentPeriod.ReportingCurrency = "USD"
	prevPeriod := makePeriod("p11", 2026, 11)
	prevPeriod.ReportingCurrency = "EUR"
	periods := []*model.BudgetPeriod{currentPeriod, prevPeriod}
	exp.set(2026, 12, []ExpenseData{{ReportingAmount: 80000}})
	exp.set(2026, 11, []ExpenseData{{ReportingAmount: 70000}})
	svc := newFanoutService(historicalRepo(periods), exp)

	result, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.NoError(t, err)
	assert.Equal(t, int64(80000), result.CurrentSpent)
	assert.Equal(t, int64(70000), result.PreviousSpent)
	assert.Equal(t, "EUR", result.PreviousReportingCurrency)
	assert.False(t, result.Comparable, "different reporting currencies are not comparable")
	assert.Equal(t, float64(0), result.ChangePercent, "change percent zeroed when not comparable")
	assert.Nil(t, result.RollingAverage, "rolling average suppressed for mixed currencies")
}

// TestGetHistoricalComparison_SameCurrencyComparable asserts that same-currency
// adjacent periods produce a Comparable=true result with a valid change percent.
func TestGetHistoricalComparison_SameCurrencyComparable(t *testing.T) {
	exp := newCountingExpenseClient()
	periods := []*model.BudgetPeriod{
		makePeriod("p12", 2026, 12),
		makePeriod("p11", 2026, 11),
	}
	exp.set(2026, 12, []ExpenseData{{ReportingAmount: 80000}})
	exp.set(2026, 11, []ExpenseData{{ReportingAmount: 70000}})
	svc := newFanoutService(historicalRepo(periods), exp)

	result, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.NoError(t, err)
	assert.True(t, result.Comparable)
	assert.InDelta(t, 14.29, result.ChangePercent, 0.01)
}

// TestGetHistoricalComparison_FanOutByteIdentical asserts the fan-out yields the
// expected values (current/previous/change/rolling) while reading each distinct
// needed period exactly once. The fan-out reads priorPeriods[0] once, and the
// unused 4th prior is never read.
func TestGetHistoricalComparison_FanOutByteIdentical(t *testing.T) {
	exp := newCountingExpenseClient()
	periods := []*model.BudgetPeriod{
		makePeriod("p12", 2026, 12),
		makePeriod("p11", 2026, 11),
		makePeriod("p10", 2026, 10),
		makePeriod("p09", 2026, 9),
		makePeriod("p08", 2026, 8),
	}
	exp.set(2026, 12, []ExpenseData{{ReportingAmount: 80000}})
	exp.set(2026, 11, []ExpenseData{{ReportingAmount: 70000}})
	exp.set(2026, 10, []ExpenseData{{ReportingAmount: 60000}})
	exp.set(2026, 9, []ExpenseData{{ReportingAmount: 50000}})
	exp.set(2026, 8, []ExpenseData{{ReportingAmount: 40000}})
	svc := newFanoutService(historicalRepo(periods), exp)

	result, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.NoError(t, err)
	assert.Equal(t, int64(80000), result.CurrentSpent)
	assert.Equal(t, int64(70000), result.PreviousSpent)
	require.NotNil(t, result.RollingAverage)
	assert.Equal(t, int64(60000), *result.RollingAverage) // (70000+60000+50000)/3
	assert.InDelta(t, 14.29, result.ChangePercent, 0.01)  // (80000-70000)/70000

	assert.Equal(t, 1, exp.counter.Count(monthOp(2026, 12)))
	assert.Equal(t, 1, exp.counter.Count(monthOp(2026, 11)), "priorPeriods[0] read once, not twice")
	assert.Equal(t, 1, exp.counter.Count(monthOp(2026, 10)))
	assert.Equal(t, 1, exp.counter.Count(monthOp(2026, 9)))
	assert.Equal(t, 0, exp.counter.Count(monthOp(2026, 8)), "4th prior is never needed, never read")
	assert.Equal(t, 4, exp.counter.Total(), "at most 4 distinct period reads")
}

// TestGetHistoricalComparison_FanOutRunsConcurrently confirms the current and
// prior reads overlap while staying within the SetLimit bound.
func TestGetHistoricalComparison_FanOutRunsConcurrently(t *testing.T) {
	exp := newCountingExpenseClient()
	exp.delay = 5 * time.Millisecond
	periods := []*model.BudgetPeriod{
		makePeriod("p12", 2026, 12), makePeriod("p11", 2026, 11),
		makePeriod("p10", 2026, 10), makePeriod("p09", 2026, 9),
	}
	for m := int32(9); m <= 12; m++ {
		exp.set(2026, m, []ExpenseData{{ReportingAmount: int64(m) * 1000}})
	}
	svc := newFanoutService(historicalRepo(periods), exp)

	_, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.NoError(t, err)
	assert.LessOrEqual(t, exp.maxConcurrent(), dashboardFanoutLimit)
	assert.Greater(t, exp.maxConcurrent(), 1, "prior reads should overlap the current read")
}

// TestGetHistoricalComparison_FanOutWrapsError confirms a prior-period read
// failure is surfaced with the period named (wrapped inside the goroutine).
func TestGetHistoricalComparison_FanOutWrapsError(t *testing.T) {
	exp := newCountingExpenseClient()
	periods := []*model.BudgetPeriod{
		makePeriod("p12", 2026, 12), makePeriod("p11", 2026, 11),
		makePeriod("p10", 2026, 10), makePeriod("p09", 2026, 9),
	}
	for m := int32(9); m <= 12; m++ {
		exp.set(2026, m, []ExpenseData{{ReportingAmount: int64(m) * 1000}})
	}
	exp.failOn(2026, 10, errors.New("upstream down"))
	svc := newFanoutService(historicalRepo(periods), exp)

	_, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2026-10", "error must name the failing period")
	assert.Contains(t, err.Error(), "upstream down")
}
