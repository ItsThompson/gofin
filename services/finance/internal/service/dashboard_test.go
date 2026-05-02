package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

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
	// 0% allocated but has spending: percent should be 100 (over budget)
	cs := buildCategorySummary(0, 5000)
	assert.Equal(t, float64(100), cs.PercentUsed)
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

	// Use a historical month so all days are elapsed
	summary := ComputePeriodSummary(period, expenses, 2025, 1)

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

	summary := ComputePeriodSummary(period, []ExpenseData{}, 2025, 3)

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

	summary := ComputePeriodSummary(period, expenses, 2025, 2)

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

	summary := ComputePeriodSummary(period, []ExpenseData{}, 2025, 4)

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
	summary := ComputePeriodSummary(period, expenses, 2025, 1)

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

	points := ComputeCumulativeSpend(expenses, 310000, 31)

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
	points := ComputeCumulativeSpend([]ExpenseData{}, 310000, 31)

	assert.Equal(t, int64(10000), points[0].Ideal)   // day 1: 310000/31*1 = 10000
	assert.Equal(t, int64(150000), points[14].Ideal)  // day 15: 310000/31*15 = 150000
	assert.Equal(t, int64(310000), points[30].Ideal)  // day 31: full budget
}

func TestComputeCumulativeSpend_NoExpenses(t *testing.T) {
	points := ComputeCumulativeSpend([]ExpenseData{}, 300000, 30)

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

	points := ComputeCumulativeSpend(expenses, 300000, 31)

	// Days 1-9: 0
	for i := 0; i < 9; i++ {
		assert.Equal(t, int64(0), points[i].Actual, "day %d should be 0", i+1)
	}
	// Days 10-31: 50000 carried forward
	for i := 9; i < 31; i++ {
		assert.Equal(t, int64(50000), points[i].Actual, "day %d should be 50000", i+1)
	}
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

func TestParseDayFromDate(t *testing.T) {
	assert.Equal(t, int32(15), parseDayFromDate("2025-01-15"))
	assert.Equal(t, int32(1), parseDayFromDate("2025-12-01"))
	assert.Equal(t, int32(0), parseDayFromDate("not-a-date"))
	assert.Equal(t, int32(0), parseDayFromDate(""))
}
