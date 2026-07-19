package service

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GetHealthScoreTrend returns the health-score trend: ascending monthly points
// for the up-to-`months` most recent budgeted periods at or before
// (year, month). Months with no budget period are skipped (only existing
// periods appear), and zero-budget months carry no score so they are skipped
// too. Each point reuses the single-month persistence policy: closed months are
// served from storage (recomputed and upserted on a miss or a stale formula
// version) and the current provisional month is computed live as the last
// point. months is clamped to [1, 12] (default 6).
//
// Steady state is cheap: every closed point is a single stored-row read. Only a
// cold cache (a month never read before) recomputes, which reuses the same
// compute-and-upsert path the single-month card warms.
func (s *FinanceService) GetHealthScoreTrend(ctx context.Context, userID string, year, month, months int32) ([]model.HealthScoreTrendPoint, error) {
	if months < 1 {
		months = 6
	}
	if months > 12 {
		months = 12
	}

	periods, err := s.repo.ListPeriods(ctx, userID) // year DESC, month DESC
	if err != nil {
		return nil, fmt.Errorf("listing periods: %w", err)
	}

	// Take the up-to-`months` most recent budgeted periods at or before the
	// target, then reverse to ascending (chronological) order for the sparkline.
	selected := make([]*model.BudgetPeriod, 0, months)
	for _, period := range periods {
		if afterTarget(period.Year, period.Month, year, month) {
			continue
		}
		if period.BudgetAmount == 0 {
			continue // no score for an unconfigured month
		}
		selected = append(selected, period)
		if int32(len(selected)) == months {
			break
		}
	}
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	points := make([]model.HealthScoreTrendPoint, len(selected))
	for i, period := range selected {
		score, err := s.resolveHealthScore(ctx, userID, period, period.Year, period.Month)
		if err != nil {
			return nil, err
		}
		points[i] = model.HealthScoreTrendPoint{
			Year:           score.Year,
			Month:          score.Month,
			Total:          score.Total,
			Band:           score.Band,
			Provisional:    score.Provisional,
			FormulaVersion: score.FormulaVersion,
		}
	}
	return points, nil
}
