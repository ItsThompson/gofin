package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GetPeriodSummary computes the full dashboard summary for a budget period.
// It fetches the period from the repository and expenses from the expense service,
// then computes totals, pacing, and category breakdowns.
func (s *FinanceService) GetPeriodSummary(ctx context.Context, userID string, year, month int32) (*model.PeriodSummary, error) {
	period, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	return ComputePeriodSummary(period, expenses, year, month, time.Now()), nil
}

// GetSpendingByTag computes per-tag spending for a budget period.
func (s *FinanceService) GetSpendingByTag(ctx context.Context, userID string, year, month int32) ([]model.TagSpending, error) {
	// Validate period exists
	_, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	tags, err := s.repo.ListTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	tagNameMap := make(map[string]string, len(tags))
	for _, tag := range tags {
		tagNameMap[tag.ID] = tag.Name
	}

	return ComputeTagSpending(expenses, tagNameMap), nil
}

// GetCumulativeSpend computes daily cumulative spending data points for the chart.
func (s *FinanceService) GetCumulativeSpend(ctx context.Context, userID string, year, month int32) ([]model.CumulativeSpendPoint, error) {
	period, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	daysInMonth := daysInMonth(year, month)
	return ComputeCumulativeSpend(expenses, period.BudgetAmount, daysInMonth), nil
}

// GetHistoricalComparison computes the historical spending comparison for a period.
// Returns current vs previous period spending, rolling 3-period average, and change percent.
func (s *FinanceService) GetHistoricalComparison(ctx context.Context, userID string, year, month int32) (*model.HistoricalComparison, error) {
	// Validate the requested period exists
	_, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	// List all periods (ordered year DESC, month DESC)
	periods, err := s.repo.ListPeriods(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing periods: %w", err)
	}

	// Find the requested period's position and collect up to 3 prior periods
	return s.computeHistoricalComparison(ctx, userID, year, month, periods)
}

// computeHistoricalComparison builds the HistoricalComparison from ordered periods.
// periods must be ordered year DESC, month DESC.
func (s *FinanceService) computeHistoricalComparison(
	ctx context.Context,
	userID string,
	year, month int32,
	periods []*model.BudgetPeriod,
) (*model.HistoricalComparison, error) {
	// Find the index of the requested period
	requestedIdx := -1
	for i, p := range periods {
		if p.Year == year && p.Month == month {
			requestedIdx = i
			break
		}
	}
	if requestedIdx == -1 {
		// Should not happen since we validated above, but be defensive
		return nil, fmt.Errorf("requested period not found in list")
	}

	// Get current period's total spent
	currentSpent, err := s.getTotalSpentForPeriod(ctx, userID, periods[requestedIdx])
	if err != nil {
		return nil, err
	}

	// Collect up to 3 prior periods (the ones after requestedIdx in DESC order)
	var priorPeriods []*model.BudgetPeriod
	for i := requestedIdx + 1; i < len(periods) && len(priorPeriods) < 3; i++ {
		priorPeriods = append(priorPeriods, periods[i])
	}

	result := &model.HistoricalComparison{
		CurrentSpent: currentSpent,
	}

	// Previous period spent
	if len(priorPeriods) > 0 {
		prevSpent, err := s.getTotalSpentForPeriod(ctx, userID, priorPeriods[0])
		if err != nil {
			return nil, err
		}
		result.PreviousSpent = prevSpent

		// Change percent
		if prevSpent > 0 {
			result.ChangePercent = math.Round(float64(currentSpent-prevSpent)/float64(prevSpent)*10000) / 100
		} else if currentSpent > 0 {
			result.ChangePercent = 100.0
		}
	}

	// Rolling average: need at least 3 prior periods
	if len(priorPeriods) >= 3 {
		var totalForAvg int64
		for _, p := range priorPeriods[:3] {
			spent, err := s.getTotalSpentForPeriod(ctx, userID, p)
			if err != nil {
				return nil, err
			}
			totalForAvg += spent
		}
		avg := totalForAvg / 3
		result.RollingAverage = &avg
	}

	return result, nil
}

// getTotalSpentForPeriod fetches expenses for a period and returns the total spent.
func (s *FinanceService) getTotalSpentForPeriod(ctx context.Context, userID string, period *model.BudgetPeriod) (int64, error) {
	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, period.Year, period.Month)
	if err != nil {
		return 0, fmt.Errorf("fetching expenses for %d-%02d: %w", period.Year, period.Month, err)
	}
	var total int64
	for _, exp := range expenses {
		total += exp.Amount
	}
	return total, nil
}

// ComputePeriodSummary is the pure computation for a period summary.
// Exported for direct testing without service/repo dependencies.
// The now parameter controls the clock for days-elapsed calculation,
// making current-month pacing deterministic in tests.
func ComputePeriodSummary(period *model.BudgetPeriod, expenses []ExpenseData, year, month int32, now time.Time) *model.PeriodSummary {
	daysInPeriod := daysInMonth(year, month)
	daysElapsed := computeDaysElapsed(year, month, daysInPeriod, now)

	// Sum expenses by type
	var totalSpent, essentialsSpent, desiresSpent, savingsSpent int64
	for _, exp := range expenses {
		totalSpent += exp.Amount
		switch exp.ExpenseType {
		case "essentials":
			essentialsSpent += exp.Amount
		case "desires":
			desiresSpent += exp.Amount
		case "savings":
			savingsSpent += exp.Amount
		}
	}

	// Compute category allocations with rounding remainder to largest
	essentialsAlloc, desiresAlloc, savingsAlloc := allocateCategories(
		period.BudgetAmount,
		period.EssentialsPercent,
		period.DesiresPercent,
		period.SavingsPercent,
	)

	remaining := period.BudgetAmount - totalSpent

	// Pacing
	var dailySpendRate int64
	if daysElapsed > 0 {
		dailySpendRate = totalSpent / int64(daysElapsed)
	}

	daysRemaining := daysInPeriod - daysElapsed
	var budgetPace int64
	if daysRemaining > 0 {
		budgetPace = remaining / int64(daysRemaining)
	}

	// On-track: compare daily spend rate to ideal daily rate
	var isOnTrack bool
	if daysInPeriod > 0 {
		idealDailyRate := float64(period.BudgetAmount) / float64(daysInPeriod)
		isOnTrack = float64(dailySpendRate) <= idealDailyRate
	} else {
		isOnTrack = true
	}

	return &model.PeriodSummary{
		PeriodID:       period.ID,
		Year:           year,
		Month:          month,
		TotalBudget:    period.BudgetAmount,
		TotalSpent:     totalSpent,
		Remaining:      remaining,
		DaysInPeriod:   daysInPeriod,
		DaysElapsed:    daysElapsed,
		DailySpendRate: dailySpendRate,
		BudgetPace:     budgetPace,
		IsOnTrack:      isOnTrack,
		Essentials:     buildCategorySummary(essentialsAlloc, essentialsSpent),
		Desires:        buildCategorySummary(desiresAlloc, desiresSpent),
		Savings:        buildCategorySummary(savingsAlloc, savingsSpent),
	}
}

// ComputeTagSpending aggregates expenses by tag and sorts by amount descending.
func ComputeTagSpending(expenses []ExpenseData, tagNames map[string]string) []model.TagSpending {
	if len(expenses) == 0 {
		return []model.TagSpending{}
	}

	// Aggregate by tag
	tagAmounts := make(map[string]int64)
	var totalSpent int64
	for _, exp := range expenses {
		tagAmounts[exp.TagID] += exp.Amount
		totalSpent += exp.Amount
	}

	// Build result
	result := make([]model.TagSpending, 0, len(tagAmounts))
	for tagID, amount := range tagAmounts {
		tagName := tagNames[tagID]
		if tagName == "" {
			tagName = "Unknown"
		}
		var pct float64
		if totalSpent > 0 {
			pct = math.Round(float64(amount)/float64(totalSpent)*10000) / 100
		}
		result = append(result, model.TagSpending{
			TagID:          tagID,
			TagName:        tagName,
			Amount:         amount,
			PercentOfTotal: pct,
		})
	}

	// Sort by amount descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Amount > result[j].Amount
	})

	return result
}

// ComputeCumulativeSpend generates daily cumulative spending points.
// Each day carries forward the previous day's total if no expenses exist on that day.
func ComputeCumulativeSpend(expenses []ExpenseData, totalBudget int64, daysInPeriod int32) []model.CumulativeSpendPoint {
	// Build day-by-day spend map
	daySpend := make(map[int32]int64)
	for _, exp := range expenses {
		day := parseDayFromDate(exp.ExpenseDate)
		if day > 0 && day <= daysInPeriod {
			daySpend[day] += exp.Amount
		}
	}

	points := make([]model.CumulativeSpendPoint, daysInPeriod)
	var cumulative int64
	for day := int32(1); day <= daysInPeriod; day++ {
		cumulative += daySpend[day]
		// Ideal linear pace: (budget / daysInPeriod) * day
		ideal := int64(math.Round(float64(totalBudget) / float64(daysInPeriod) * float64(day)))
		points[day-1] = model.CumulativeSpendPoint{
			Day:    day,
			Actual: cumulative,
			Ideal:  ideal,
		}
	}

	return points
}

// allocateCategories distributes the budget across E/D/S categories.
// When the percentages don't divide evenly, the largest category absorbs
// the rounding remainder per spec.
func allocateCategories(budget int64, essentialsPct, desiresPct, savingsPct int32) (int64, int64, int64) {
	if budget == 0 {
		return 0, 0, 0
	}

	essentials := budget * int64(essentialsPct) / 100
	desires := budget * int64(desiresPct) / 100
	savings := budget * int64(savingsPct) / 100

	remainder := budget - (essentials + desires + savings)
	if remainder == 0 {
		return essentials, desires, savings
	}

	// Assign remainder to largest category
	maxPct := essentialsPct
	target := "essentials"
	if desiresPct > maxPct {
		maxPct = desiresPct
		target = "desires"
	}
	if savingsPct > maxPct {
		target = "savings"
	}

	switch target {
	case "essentials":
		essentials += remainder
	case "desires":
		desires += remainder
	case "savings":
		savings += remainder
	}

	return essentials, desires, savings
}

// buildCategorySummary creates a CategorySummary from allocated and spent amounts.
func buildCategorySummary(allocated, spent int64) model.CategorySummary {
	remaining := allocated - spent
	var percentUsed float64
	if allocated > 0 {
		percentUsed = math.Round(float64(spent)/float64(allocated)*10000) / 100
	} else if spent > 0 {
		// 0% allocated but has spending: any amount is infinitely over budget.
		// Report as 100+ so the frontend's >= 100 check triggers the over-budget indicator.
		percentUsed = 100 + math.Round(float64(spent)/100)
	}
	return model.CategorySummary{
		Allocated:   allocated,
		Spent:       spent,
		Remaining:   remaining,
		PercentUsed: percentUsed,
	}
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year, month int32) int32 {
	// time.Date with day=0 of the next month gives the last day of this month.
	lastDay := time.Date(int(year), time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
	return int32(lastDay.Day())
}

// computeDaysElapsed determines how many days have elapsed in the period.
// If the period matches now's month, returns now's day. Otherwise returns
// the full day count (period is fully elapsed).
func computeDaysElapsed(year, month, daysInPeriod int32, now time.Time) int32 {
	if int32(now.Year()) == year && int32(now.Month()) == month {
		return int32(now.Day())
	}
	return daysInPeriod
}

// parseDayFromDate extracts the day-of-month from a "YYYY-MM-DD" date string.
// Returns 0 on parse failure.
func parseDayFromDate(date string) int32 {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	return int32(t.Day())
}
