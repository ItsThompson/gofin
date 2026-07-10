package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/perf"
)

// historicalNow is a fixed time far in the future, ensuring any test month
// is treated as a past (fully elapsed) period.
var historicalNow = time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

// --- Category Allocation Tests ---

func TestAllocateCategories_EvenSplit(t *testing.T) {
	// $3000 budget with 50/30/20 split: $1500/$900/$600 (no remainder)
	e, d, s := allocateCategories(300000, 50, 30, 20)
	assert.Equal(t, int64(150000), e)
	assert.Equal(t, int64(90000), d)
	assert.Equal(t, int64(60000), s)
}

func TestAllocateCategories_RemainderToLargest(t *testing.T) {
	// $100.01 (10001 cents) with 33/33/34: truncated = 3300 + 3300 + 3400 = 10000
	// remainder = 1 cent, assigned to savings (34% is largest)
	e, d, s := allocateCategories(10001, 33, 33, 34)
	assert.Equal(t, int64(3300), e)
	assert.Equal(t, int64(3300), d)
	assert.Equal(t, int64(3401), s) // 3400 + 1 remainder
	assert.Equal(t, int64(10001), e+d+s, "must sum to budget")
}

func TestAllocateCategories_RemainderToEssentials(t *testing.T) {
	// Budget = 101 cents, 50/30/20 → 50 + 30 + 20 = 100, remainder 1 → essentials
	e, d, s := allocateCategories(101, 50, 30, 20)
	assert.Equal(t, int64(51), e) // 50 + 1
	assert.Equal(t, int64(30), d)
	assert.Equal(t, int64(20), s)
	assert.Equal(t, int64(101), e+d+s)
}

func TestAllocateCategories_ZeroBudget(t *testing.T) {
	e, d, s := allocateCategories(0, 50, 30, 20)
	assert.Equal(t, int64(0), e)
	assert.Equal(t, int64(0), d)
	assert.Equal(t, int64(0), s)
}

func TestAllocateCategories_AllToOneCategory(t *testing.T) {
	e, d, s := allocateCategories(300000, 100, 0, 0)
	assert.Equal(t, int64(300000), e)
	assert.Equal(t, int64(0), d)
	assert.Equal(t, int64(0), s)
}

// --- Category Summary Tests ---

func TestBuildCategorySummary_Normal(t *testing.T) {
	cs := buildCategorySummary(150000, 75000)
	assert.Equal(t, int64(150000), cs.Allocated)
	assert.Equal(t, int64(75000), cs.Spent)
	assert.Equal(t, int64(75000), cs.Remaining)
	assert.InDelta(t, 50.0, cs.PercentUsed, 0.01)
}

func TestBuildCategorySummary_OverBudget(t *testing.T) {
	cs := buildCategorySummary(100000, 150000)
	assert.Equal(t, int64(-50000), cs.Remaining)
	assert.InDelta(t, 150.0, cs.PercentUsed, 0.01)
}

func TestBuildCategorySummary_ZeroAllocatedWithSpending(t *testing.T) {
	// 0% allocated but has spending: percentUsed must be > 100 to trigger over-budget.
	cs := buildCategorySummary(0, 5000)
	assert.Greater(t, cs.PercentUsed, float64(100))
}

func TestBuildCategorySummary_ZeroAllocatedNoSpending(t *testing.T) {
	cs := buildCategorySummary(0, 0)
	assert.Equal(t, float64(0), cs.PercentUsed)
}

// --- Period Summary Computation Tests ---

func TestComputePeriodSummary_BasicScenario(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      300000, // $3000
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	expenses := []ExpenseData{
		{ID: "e1", Amount: 50000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-01-05"},
		{ID: "e2", Amount: 20000, ExpenseType: "desires", TagID: "t2", ExpenseDate: "2025-01-06"},
		{ID: "e3", Amount: 10000, ExpenseType: "savings", TagID: "t3", ExpenseDate: "2025-01-07"},
	}

	summary := ComputePeriodSummary(period, expenses, 2025, 1, historicalNow)

	assert.Equal(t, "period-1", summary.PeriodID)
	assert.Equal(t, int64(300000), summary.TotalBudget)
	assert.Equal(t, int64(80000), summary.TotalSpent)
	assert.Equal(t, int64(220000), summary.Remaining)
	assert.Equal(t, int32(31), summary.DaysInPeriod) // January has 31 days

	// Category breakdowns
	assert.Equal(t, int64(150000), summary.Essentials.Allocated)
	assert.Equal(t, int64(50000), summary.Essentials.Spent)
	assert.Equal(t, int64(100000), summary.Essentials.Remaining)

	assert.Equal(t, int64(90000), summary.Desires.Allocated)
	assert.Equal(t, int64(20000), summary.Desires.Spent)

	assert.Equal(t, int64(60000), summary.Savings.Allocated)
	assert.Equal(t, int64(10000), summary.Savings.Spent)
}

func TestComputePeriodSummary_NoExpenses(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	summary := ComputePeriodSummary(period, []ExpenseData{}, 2025, 3, historicalNow)

	assert.Equal(t, int64(0), summary.TotalSpent)
	assert.Equal(t, int64(300000), summary.Remaining)
	assert.True(t, summary.IsOnTrack)
	assert.Equal(t, int64(0), summary.DailySpendRate)
}

func TestComputePeriodSummary_OverBudget(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      100000, // $1000
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	expenses := []ExpenseData{
		{ID: "e1", Amount: 120000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-02-15"},
	}

	summary := ComputePeriodSummary(period, expenses, 2025, 2, historicalNow)

	assert.Equal(t, int64(120000), summary.TotalSpent)
	assert.Equal(t, int64(-20000), summary.Remaining)
	assert.False(t, summary.IsOnTrack)
}

func TestComputePeriodSummary_ZeroBudget(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      0,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	summary := ComputePeriodSummary(period, []ExpenseData{}, 2025, 4, historicalNow)

	assert.Equal(t, int64(0), summary.TotalBudget)
	assert.Equal(t, int64(0), summary.TotalSpent)
	assert.True(t, summary.IsOnTrack)
	// Zero budget: all allocations are 0
	assert.Equal(t, int64(0), summary.Essentials.Allocated)
	assert.Equal(t, int64(0), summary.Desires.Allocated)
	assert.Equal(t, int64(0), summary.Savings.Allocated)
}

func TestComputePeriodSummary_PacingCalculation(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      310000, // $3100 — 31 days, $100/day ideal
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	// Spend $1500 in first 15 days → $100/day rate → exactly on track
	expenses := []ExpenseData{
		{ID: "e1", Amount: 150000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-01-15"},
	}

	// Historical month: all 31 days elapsed
	summary := ComputePeriodSummary(period, expenses, 2025, 1, historicalNow)

	assert.Equal(t, int32(31), summary.DaysElapsed) // historical
	// dailySpendRate = 150000 / 31 = 4838 cents
	assert.Equal(t, int64(4838), summary.DailySpendRate)
	// idealRate = 310000 / 31 = 10000 cents/day
	// 4838 <= 10000 → on track
	assert.True(t, summary.IsOnTrack)
}

// --- Tag Spending Tests ---

func TestComputeTagSpending_MultipleTagsSorted(t *testing.T) {
	expenses := []ExpenseData{
		{TagID: "food", Amount: 30000},
		{TagID: "food", Amount: 20000},
		{TagID: "transport", Amount: 10000},
		{TagID: "bills", Amount: 40000},
	}

	tagNames := map[string]string{
		"food":      "Food",
		"transport": "Transport",
		"bills":     "Bills",
	}

	result := ComputeTagSpending(expenses, tagNames)

	assert.Len(t, result, 3)
	// Sorted by amount descending: Food (50000), Bills (40000), Transport (10000)
	assert.Equal(t, "Food", result[0].TagName)
	assert.Equal(t, int64(50000), result[0].Amount)
	assert.Equal(t, "Bills", result[1].TagName)
	assert.Equal(t, int64(40000), result[1].Amount)
	assert.Equal(t, "Transport", result[2].TagName)
	assert.Equal(t, int64(10000), result[2].Amount)

	// Percentages: 50000/100000 = 50%, 40000/100000 = 40%, 10000/100000 = 10%
	assert.InDelta(t, 50.0, result[0].PercentOfTotal, 0.01)
	assert.InDelta(t, 40.0, result[1].PercentOfTotal, 0.01)
	assert.InDelta(t, 10.0, result[2].PercentOfTotal, 0.01)
}

func TestComputeTagSpending_NoExpenses(t *testing.T) {
	result := ComputeTagSpending([]ExpenseData{}, map[string]string{})
	assert.Empty(t, result)
}

func TestComputeTagSpending_UnknownTag(t *testing.T) {
	expenses := []ExpenseData{
		{TagID: "deleted-tag", Amount: 5000},
	}

	result := ComputeTagSpending(expenses, map[string]string{})
	assert.Len(t, result, 1)
	assert.Equal(t, "Unknown", result[0].TagName)
}

// --- Cumulative Spend Tests ---

func TestComputeCumulativeSpend_BasicAccumulation(t *testing.T) {
	expenses := []ExpenseData{
		{ExpenseDate: "2025-01-01", Amount: 10000},
		{ExpenseDate: "2025-01-01", Amount: 5000},
		{ExpenseDate: "2025-01-03", Amount: 20000},
	}

	points := ComputeCumulativeSpend(expenses, 310000, 2025, 1, 31)

	assert.Len(t, points, 31)
	// Day 1: 15000
	assert.Equal(t, int32(1), points[0].Day)
	assert.Equal(t, int64(15000), points[0].Actual)
	// Day 2: still 15000 (no expenses)
	assert.Equal(t, int64(15000), points[1].Actual)
	// Day 3: 15000 + 20000 = 35000
	assert.Equal(t, int64(35000), points[2].Actual)
	// Day 31: still 35000 (no more expenses)
	assert.Equal(t, int64(35000), points[30].Actual)
}

func TestComputeCumulativeSpend_IdealLine(t *testing.T) {
	// 310000 budget, 31 days → 10000/day ideal
	points := ComputeCumulativeSpend([]ExpenseData{}, 310000, 2025, 1, 31)

	assert.Equal(t, int64(10000), points[0].Ideal)   // day 1: 310000/31*1 = 10000
	assert.Equal(t, int64(150000), points[14].Ideal)  // day 15: 310000/31*15 = 150000
	assert.Equal(t, int64(310000), points[30].Ideal)  // day 31: full budget
}

func TestComputeCumulativeSpend_NoExpenses(t *testing.T) {
	points := ComputeCumulativeSpend([]ExpenseData{}, 300000, 2025, 6, 30)

	assert.Len(t, points, 30)
	for _, point := range points {
		assert.Equal(t, int64(0), point.Actual)
		assert.Greater(t, point.Ideal, int64(0))
	}
}

func TestComputeCumulativeSpend_DayCarryForward(t *testing.T) {
	expenses := []ExpenseData{
		{ExpenseDate: "2025-03-10", Amount: 50000},
	}

	points := ComputeCumulativeSpend(expenses, 300000, 2025, 3, 31)

	// Days 1-9: 0
	for i := 0; i < 9; i++ {
		assert.Equal(t, int64(0), points[i].Actual, "day %d should be 0", i+1)
	}
	// Days 10-31: 50000 carried forward
	for i := 9; i < 31; i++ {
		assert.Equal(t, int64(50000), points[i].Actual, "day %d should be 50000", i+1)
	}
}

func TestComputeCumulativeSpend_CrossMonthClamp(t *testing.T) {
	// April 29 expense assigned to May period should be clamped to day 1
	expenses := []ExpenseData{
		{ExpenseDate: "2025-04-29", Amount: 15000},
		{ExpenseDate: "2025-05-03", Amount: 20000},
	}

	points := ComputeCumulativeSpend(expenses, 300000, 2025, 5, 31)

	assert.Len(t, points, 31)
	// Day 1: April 29 expense clamped here → 15000
	assert.Equal(t, int64(15000), points[0].Actual)
	// Day 2: still 15000 (no expenses)
	assert.Equal(t, int64(15000), points[1].Actual)
	// Day 3: 15000 + 20000 = 35000
	assert.Equal(t, int64(35000), points[2].Actual)
	// Day 29: should NOT have any bump (the Apr 29 expense is at day 1, not day 29)
	assert.Equal(t, int64(35000), points[28].Actual)
}

// --- Helper Tests ---

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year, month int32
		expected    int32
	}{
		{2025, 1, 31},  // January
		{2025, 2, 28},  // February non-leap
		{2024, 2, 29},  // February leap year
		{2025, 4, 30},  // April
		{2025, 12, 31}, // December
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, daysInMonth(tt.year, tt.month))
		})
	}
}

func TestParseDayForPeriod(t *testing.T) {
	// Matching month: returns actual day
	assert.Equal(t, int32(15), parseDayForPeriod("2025-01-15", 2025, 1))

	// Mismatched month: clamped to day 1
	assert.Equal(t, int32(1), parseDayForPeriod("2025-04-29", 2025, 5))

	// Mismatched year: clamped to day 1
	assert.Equal(t, int32(1), parseDayForPeriod("2024-01-15", 2025, 1))

	// Parse failure: returns 0
	assert.Equal(t, int32(0), parseDayForPeriod("not-a-date", 2025, 1))
	assert.Equal(t, int32(0), parseDayForPeriod("", 2025, 1))
}

// --- Current-month pacing test (uses injected now) ---

func TestComputePeriodSummary_CurrentMonthPacing(t *testing.T) {
	period := &model.BudgetPeriod{
		ID:                "period-1",
		BudgetAmount:      310000, // $3100 — 31 days, $100/day ideal
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	}

	expenses := []ExpenseData{
		{ID: "e1", Amount: 100000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-01-05"},
	}

	// Simulate being on Jan 10: 10 days elapsed, spent 100000
	now := time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)
	summary := ComputePeriodSummary(period, expenses, 2025, 1, now)

	assert.Equal(t, int32(10), summary.DaysElapsed)
	// dailySpendRate = 100000 / 10 = 10000
	assert.Equal(t, int64(10000), summary.DailySpendRate)
	// idealRate = 310000 / 31 = 10000 → exactly on track
	assert.True(t, summary.IsOnTrack)
	// budgetPace = (310000 - 100000) / (31 - 10) = 210000 / 21 = 10000
	assert.Equal(t, int64(10000), summary.BudgetPace)
	// daysRemaining = 31 - 10 = 21
	assert.Equal(t, int32(21), summary.DaysInPeriod-summary.DaysElapsed)
}

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
			{Amount: 100000, ExpenseType: "essentials"},
			{Amount: 50000, ExpenseType: "desires"},
			{Amount: 20000, ExpenseType: "savings"},
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
			{Amount: 80000, ExpenseType: "essentials"},
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
		{{Amount: 5000, ExpenseType: "essentials"}},
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
			{Amount: 40000, ExpenseType: "essentials"},
			{Amount: 30000, ExpenseType: "desires"},
			{Amount: 10000, ExpenseType: "savings"},
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

// --- Fan-out test infrastructure (shared by regression tests and benchmarks) ---

// periodKey is the {year, month} lookup key used across the fan-out fakes.
func periodKey(year, month int32) [2]int32 { return [2]int32{year, month} }

// monthOp is the CallCounter operation name for a period read.
func monthOp(year, month int32) string { return fmt.Sprintf("%d-%02d", year, month) }

// countingExpenseClient is a concurrency-aware fake ExpenseClient shared by the
// dashboard fan-out regression tests and benchmarks. It records one call per
// (year, month) through an embedded *perf.CallCounter, tracks the maximum number
// of reads in flight simultaneously (so the SetLimit bound can be asserted), can
// inject per-period errors (exercising the goroutine error-wrapping path), and
// can simulate per-read latency so benchmarks show fan-out (max) rather than
// serial (sum) wall-clock. All methods are safe for concurrent use.
type countingExpenseClient struct {
	counter *perf.CallCounter
	delay   time.Duration
	byMonth map[[2]int32][]ExpenseData
	errs    map[[2]int32]error

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func newCountingExpenseClient() *countingExpenseClient {
	return &countingExpenseClient{
		counter: perf.NewCallCounter(),
		byMonth: make(map[[2]int32][]ExpenseData),
		errs:    make(map[[2]int32]error),
	}
}

// set seeds the expenses returned for a period. Call before the fan-out runs;
// the maps are read-only during concurrent reads.
func (c *countingExpenseClient) set(year, month int32, expenses []ExpenseData) {
	c.byMonth[periodKey(year, month)] = expenses
}

// failOn makes the read for a period return err, exercising the goroutine
// error-wrapping path. Call before the fan-out runs.
func (c *countingExpenseClient) failOn(year, month int32, err error) {
	c.errs[periodKey(year, month)] = err
}

func (c *countingExpenseClient) GetExpensesForPeriod(_ context.Context, _ string, year, month int32) ([]ExpenseData, error) {
	c.counter.Record(monthOp(year, month))

	c.mu.Lock()
	c.inFlight++
	if c.inFlight > c.maxInFlight {
		c.maxInFlight = c.inFlight
	}
	c.mu.Unlock()

	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	c.mu.Lock()
	c.inFlight--
	c.mu.Unlock()

	if err := c.errs[periodKey(year, month)]; err != nil {
		return nil, err
	}
	return c.byMonth[periodKey(year, month)], nil
}

// maxConcurrent reports the peak number of reads observed in flight at once.
func (c *countingExpenseClient) maxConcurrent() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxInFlight
}

// CountExpensesByTag and CreateExpense are not used by the dashboard read paths;
// they fail loudly if a path unexpectedly calls them.
func (c *countingExpenseClient) CountExpensesByTag(context.Context, string, string) (int64, error) {
	return 0, fmt.Errorf("CountExpensesByTag not expected in dashboard fan-out tests")
}

func (c *countingExpenseClient) CreateExpense(context.Context, CreateExpenseInput) (*CreatedExpenseData, error) {
	return nil, fmt.Errorf("CreateExpense not expected in dashboard fan-out tests")
}

// fakeFanoutRepo returns canned periods for the dashboard read paths. Only the
// two methods those paths call (ListPeriods, GetCurrentPeriod) are implemented;
// the embedded interface is nil, so any other repo call panics and surfaces an
// accidental extra read.
type fakeFanoutRepo struct {
	repository.FinanceRepository
	periods []*model.BudgetPeriod
	current map[[2]int32]*model.BudgetPeriod
}

func (r *fakeFanoutRepo) ListPeriods(context.Context, string) ([]*model.BudgetPeriod, error) {
	return r.periods, nil
}

func (r *fakeFanoutRepo) GetCurrentPeriod(_ context.Context, _ string, year, month int32) (*model.BudgetPeriod, error) {
	return r.current[periodKey(year, month)], nil
}

// newFanoutService wires a FinanceService for fan-out tests/benchmarks with a
// discarded logger and no transaction beginner (the read paths never open a tx).
func newFanoutService(repo repository.FinanceRepository, exp ExpenseClient) *FinanceService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewFinanceService(repo, nil, logger).WithExpenseClient(exp)
}

// --- GetSpendingTrends fan-out regression tests ---

// TestGetSpendingTrends_FanOutByteIdentical seeds a full 2026 window with two
// gaps (no period for April/September) and asserts the fan-out produces the
// serial result: chronological order preserved, per-month aggregation correct,
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
			{Amount: int64(m) * 1000, ExpenseType: "essentials"},
			{Amount: int64(m) * 500, ExpenseType: "desires"},
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

// --- GetHistoricalComparison fan-out regression tests ---

func historicalRepo(periods []*model.BudgetPeriod) *fakeFanoutRepo {
	return &fakeFanoutRepo{
		periods: periods,
		current: map[[2]int32]*model.BudgetPeriod{periodKey(periods[0].Year, periods[0].Month): periods[0]},
	}
}

// TestGetHistoricalComparison_FanOutByteIdentical asserts the fan-out yields the
// serial values (current/previous/change/rolling) while reading each distinct
// needed period exactly once. The serial path read priorPeriods[0] twice; the
// fan-out reads it once, and the unused 4th prior is never read.
func TestGetHistoricalComparison_FanOutByteIdentical(t *testing.T) {
	exp := newCountingExpenseClient()
	periods := []*model.BudgetPeriod{
		makePeriod("p12", 2026, 12),
		makePeriod("p11", 2026, 11),
		makePeriod("p10", 2026, 10),
		makePeriod("p09", 2026, 9),
		makePeriod("p08", 2026, 8),
	}
	exp.set(2026, 12, []ExpenseData{{Amount: 80000}})
	exp.set(2026, 11, []ExpenseData{{Amount: 70000}})
	exp.set(2026, 10, []ExpenseData{{Amount: 60000}})
	exp.set(2026, 9, []ExpenseData{{Amount: 50000}})
	exp.set(2026, 8, []ExpenseData{{Amount: 40000}})
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
		exp.set(2026, m, []ExpenseData{{Amount: int64(m) * 1000}})
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
		exp.set(2026, m, []ExpenseData{{Amount: int64(m) * 1000}})
	}
	exp.failOn(2026, 10, errors.New("upstream down"))
	svc := newFanoutService(historicalRepo(periods), exp)

	_, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2026-10", "error must name the failing period")
	assert.Contains(t, err.Error(), "upstream down")
}
