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

// BudgetPeriodsProvider fetches budget period data from the finance service.
type BudgetPeriodsProvider struct {
	financeClient financepb.FinanceServiceClient
}

// NewBudgetPeriodsProvider creates a BudgetPeriodsProvider backed by the finance gRPC client.
func NewBudgetPeriodsProvider(financeClient financepb.FinanceServiceClient) *BudgetPeriodsProvider {
	return &BudgetPeriodsProvider{financeClient: financeClient}
}

// Name returns the CSV filename for this provider.
func (p *BudgetPeriodsProvider) Name() string {
	return "budget_periods"
}

// Headers returns the CSV column headers for budget period data.
func (p *BudgetPeriodsProvider) Headers() []string {
	return []string{
		"id", "year", "month", "budget_amount",
		"essentials_percent", "desires_percent", "savings_percent", "created_at",
	}
}

// Collect fetches all budget periods for the user and returns formatted rows.
func (p *BudgetPeriodsProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	resp, err := p.financeClient.GetAllUserData(ctx, &financepb.GetAllUserDataRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("fetching user data for budget periods: %w", err)
	}

	periods := resp.GetPeriods()
	rows := make([][]string, 0, len(periods))
	for _, period := range periods {
		rows = append(rows, []string{
			period.GetId(),
			strconv.FormatInt(int64(period.GetYear()), 10),
			strconv.FormatInt(int64(period.GetMonth()), 10),
			formatCentsToDollars(period.GetBudgetAmount()),
			strconv.FormatInt(int64(period.GetEssentialsPercent()), 10),
			strconv.FormatInt(int64(period.GetDesiresPercent()), 10),
			strconv.FormatInt(int64(period.GetSavingsPercent()), 10),
			period.GetCreatedAt(),
		})
	}

	return rows, nil
}
