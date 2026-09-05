package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that BudgetPeriodsProvider implements DataProvider.
var _ engine.DataProvider = (*BudgetPeriodsProvider)(nil)

// BudgetPeriodsProvider maps the budget periods in the shared per-job finance
// response into rows.
type BudgetPeriodsProvider struct {
	data *financepb.AllUserDataResponse
}

// NewBudgetPeriodsProvider creates a BudgetPeriodsProvider over the finance data
// the export engine fetches once per job.
func NewBudgetPeriodsProvider(data *financepb.AllUserDataResponse) *BudgetPeriodsProvider {
	return &BudgetPeriodsProvider{data: data}
}

// Name returns the CSV filename for this provider.
func (p *BudgetPeriodsProvider) Name() string {
	return "budget_periods"
}

// Headers returns the CSV column headers for budget period data.
func (p *BudgetPeriodsProvider) Headers() []string {
	return []string{
		"id", "year", "month", "budget_amount", "reporting_currency",
		"essentials_percent", "desires_percent", "savings_percent", "created_at",
	}
}

// Collect maps the pre-fetched user data's budget periods into rows. It is a
// pure mapper: the finance fetch happens once in the export engine.
func (p *BudgetPeriodsProvider) Collect(_ context.Context, _ string) ([][]string, error) {
	periods := p.data.GetPeriods()
	rows := make([][]string, 0, len(periods))
	for _, period := range periods {
		budgetAmount, err := formatMinorUnits(period.GetBudgetAmount(), period.GetReportingCurrencyCode())
		if err != nil {
			return nil, fmt.Errorf("period %s budget amount: %w", period.GetId(), err)
		}
		rows = append(rows, []string{
			period.GetId(),
			strconv.FormatInt(int64(period.GetYear()), 10),
			strconv.FormatInt(int64(period.GetMonth()), 10),
			budgetAmount,
			period.GetReportingCurrencyCode(),
			strconv.FormatInt(int64(period.GetEssentialsPercent()), 10),
			strconv.FormatInt(int64(period.GetDesiresPercent()), 10),
			strconv.FormatInt(int64(period.GetSavingsPercent()), 10),
			period.GetCreatedAt(),
		})
	}

	return rows, nil
}
