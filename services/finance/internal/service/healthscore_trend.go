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
// too. months is clamped to [1, 12] (default 6).
//
// Stored closed months at the current formula version are read in a single
// batched scalar query (total/band/formula_version only, no score JSONB), served
// by idx_health_scores_user. Only a cold cache (a month never read, or a stale
// version) or the current provisional month falls back to the shared
// compute-and-upsert path (resolveHealthScore), which the single-month card also
// warms.
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

	scalars, err := s.repo.ListHealthScoreScalars(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing health score scalars: %w", err)
	}
	scalarByMonth := make(map[[2]int32]*model.HealthScoreTrendPoint, len(scalars))
	for _, scalar := range scalars {
		scalarByMonth[[2]int32{scalar.Year, scalar.Month}] = scalar
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

	now := s.nowFunc()
	points := make([]model.HealthScoreTrendPoint, len(selected))
	for i, period := range selected {
		// A stored current-version scalar for a closed month is used directly (no
		// JSONB deserialize, no compute). The provisional month is never stored, so
		// it always falls through to a live compute.
		if !isProvisional(period.Year, period.Month, now) {
			if scalar := scalarByMonth[[2]int32{period.Year, period.Month}]; scalar != nil && scalar.FormulaVersion == model.FormulaVersion {
				points[i] = *scalar
				points[i].ReportingCurrency = period.ReportingCurrency
				continue
			}
		}

		score, err := s.resolveHealthScore(ctx, userID, period, period.Year, period.Month)
		if err != nil {
			return nil, err
		}
		points[i] = model.HealthScoreTrendPoint{
			Year:              score.Year,
			Month:             score.Month,
			Total:             score.Total,
			Band:              score.Band,
			Provisional:       score.Provisional,
			FormulaVersion:    score.FormulaVersion,
			ReportingCurrency: period.ReportingCurrency,
		}
	}
	return points, nil
}
