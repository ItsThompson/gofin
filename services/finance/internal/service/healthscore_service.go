package service

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GetHealthScore returns the monthly financial health score. It fetches the
// period (404 PERIOD_NOT_FOUND when none exists) and short-circuits the
// zero-budget case with a configure-budget response, then applies the
// persistence policy in resolveHealthScore.
func (s *FinanceService) GetHealthScore(ctx context.Context, userID string, year, month int32) (*model.HealthScore, error) {
	period, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}
	if period.BudgetAmount == 0 {
		return &model.HealthScore{Year: year, Month: month, ConfigureBudget: true, ReportingCurrencyCode: period.ReportingCurrencyCode}, nil
	}
	return s.resolveHealthScore(ctx, userID, period, year, month)
}

// resolveHealthScore applies the persistence policy for one budgeted month. The
// provisional (current) month is computed live and never stored, since its
// month-to-date data is not yet final. A stored closed month at the current
// formula version is returned as-is (a complete score including its insight, no
// recompute and no expense read); a missing or stale-version closed month is
// recomputed and upserted in place (lazy backfill). period.BudgetAmount must be
// greater than 0.
func (s *FinanceService) resolveHealthScore(ctx context.Context, userID string, period *model.BudgetPeriod, year, month int32) (*model.HealthScore, error) {
	if isProvisional(year, month, s.nowFunc()) {
		return s.computeHealthScore(ctx, userID, period, year, month)
	}

	stored, err := s.repo.GetHealthScore(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("getting stored health score: %w", err)
	}
	if stored != nil && stored.FormulaVersion == model.FormulaVersion {
		// Backfill ReportingCurrency for scores persisted before the field was
		// added. The period's reporting currency is immutable, so this is safe.
		if stored.ReportingCurrencyCode == "" {
			stored.ReportingCurrencyCode = period.ReportingCurrencyCode
		}
		return stored, nil
	}

	result, err := s.computeHealthScore(ctx, userID, period, year, month)
	if err != nil {
		return nil, err
	}
	// Persisting is best-effort: a write failure must not fail the read, so it is
	// logged and the freshly computed score is still returned.
	if _, err := s.repo.UpsertHealthScore(ctx, userID, result); err != nil {
		s.logger.Error("upserting health score",
			slog.String("method", "resolveHealthScore"),
			slog.String("user_id", userID),
			slog.Int("year", int(year)),
			slog.Int("month", int(month)),
			slog.String("error", err.Error()),
		)
	}
	return result, nil
}

// computeHealthScore reads the inputs the pure ComputeHealthScore needs: the
// target month's expenses and the desires window that feeds the stability
// sub-score. The insight currency is the scored period's immutable reporting
// currency, not the user's default settings currency, so the display stays
// stable after default settings changes.
func (s *FinanceService) computeHealthScore(ctx context.Context, userID string, period *model.BudgetPeriod, year, month int32) (*model.HealthScore, error) {
	currency := period.ReportingCurrencyCode
	if currency == "" {
		currency = defaultCurrency
	}

	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	desiresWindow, err := s.buildDesiresWindow(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	return ComputeHealthScore(period, expenses, desiresWindow, year, month, s.nowFunc(), currency), nil
}

// buildDesiresWindow returns the discretionary (desires) totals for the up-to-
// stabilityWindowMonths most recent CLOSED months at or before (year, month),
// which feed the stability sub-score. The current provisional month is excluded
// (its month-to-date desires would swing the CoV and break determinism), so
// a provisional target's window is the trailing closed months. The per-period
// desires reads are independent and fan out under the shared dashboard limit.
func (s *FinanceService) buildDesiresWindow(ctx context.Context, userID string, year, month int32) ([]int64, error) {
	periods, err := s.repo.ListPeriods(ctx, userID) // year DESC, month DESC
	if err != nil {
		return nil, fmt.Errorf("listing periods: %w", err)
	}

	now := s.nowFunc()
	selected := make([]*model.BudgetPeriod, 0, stabilityWindowMonths)
	for _, period := range periods {
		if afterTarget(period.Year, period.Month, year, month) {
			continue
		}
		if isProvisional(period.Year, period.Month, now) {
			continue // exclude the current open month
		}
		selected = append(selected, period)
		if len(selected) == stabilityWindowMonths {
			break
		}
	}

	desires := make([]int64, len(selected))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(dashboardFanoutLimit)
	for i, period := range selected {
		i, period := i, period
		g.Go(s.guardFanout(gctx, "health score desires window", userID, func() error {
			expenses, err := s.expenseClient.GetExpensesForPeriod(gctx, userID, period.Year, period.Month)
			if err != nil {
				return fmt.Errorf("fetching desires for %d-%02d: %w", period.Year, period.Month, err)
			}
			desires[i] = sumDesires(expenses)
			return nil
		}, periodAttr(period.Year, period.Month)))
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return desires, nil
}

// afterTarget reports whether (year, month) is strictly after (targetYear,
// targetMonth) in calendar order.
func afterTarget(year, month, targetYear, targetMonth int32) bool {
	return year > targetYear || (year == targetYear && month > targetMonth)
}

// sumDesires totals the active desires expenses in a period.
func sumDesires(expenses []ExpenseData) int64 {
	var total int64
	for _, expense := range expenses {
		if expense.ExpenseType == "desires" {
			total += expense.ReportingAmount
		}
	}
	return total
}
