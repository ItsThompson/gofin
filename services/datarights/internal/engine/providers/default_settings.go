package providers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that DefaultSettingsProvider implements DataProvider.
var _ engine.DataProvider = (*DefaultSettingsProvider)(nil)

// DefaultSettingsProvider fetches the user's default financial settings from the finance service.
type DefaultSettingsProvider struct {
	financeClient financepb.FinanceServiceClient
}

// NewDefaultSettingsProvider creates a DefaultSettingsProvider backed by the finance gRPC client.
func NewDefaultSettingsProvider(financeClient financepb.FinanceServiceClient) *DefaultSettingsProvider {
	return &DefaultSettingsProvider{financeClient: financeClient}
}

// Name returns the CSV filename for this provider.
func (p *DefaultSettingsProvider) Name() string {
	return "default_settings"
}

// Headers returns the CSV column headers for default settings data.
func (p *DefaultSettingsProvider) Headers() []string {
	return []string{
		"budget_amount", "essentials_percent", "desires_percent",
		"savings_percent", "currency",
	}
}

// Collect fetches the user's default settings and returns a single row.
// Returns an empty result (headers only) if no defaults are configured.
func (p *DefaultSettingsProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	resp, err := p.financeClient.GetAllUserData(ctx, &financepb.GetAllUserDataRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("fetching user data for default settings: %w", err)
	}

	defaults := resp.GetDefaults()
	if defaults == nil {
		return [][]string{}, nil
	}

	row := []string{
		formatCentsToDollars(defaults.GetBudgetAmount()),
		strconv.FormatInt(int64(defaults.GetEssentialsPercent()), 10),
		strconv.FormatInt(int64(defaults.GetDesiresPercent()), 10),
		strconv.FormatInt(int64(defaults.GetSavingsPercent()), 10),
		defaults.GetCurrency(),
	}

	return [][]string{row}, nil
}
