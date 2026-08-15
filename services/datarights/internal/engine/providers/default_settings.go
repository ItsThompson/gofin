package providers

import (
	"context"
	"strconv"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that DefaultSettingsProvider implements DataProvider.
var _ engine.DataProvider = (*DefaultSettingsProvider)(nil)

// DefaultSettingsProvider maps the default financial settings in the shared
// per-job finance response into a single row.
type DefaultSettingsProvider struct {
	data *financepb.AllUserDataResponse
}

// NewDefaultSettingsProvider creates a DefaultSettingsProvider over the finance
// data the export engine fetches once per job.
func NewDefaultSettingsProvider(data *financepb.AllUserDataResponse) *DefaultSettingsProvider {
	return &DefaultSettingsProvider{data: data}
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

// Collect maps the pre-fetched user data's default settings into a single row,
// or an empty result (headers only) if no defaults are configured. It is a pure
// mapper: the finance fetch happens once in the export engine.
func (p *DefaultSettingsProvider) Collect(_ context.Context, _ string) ([][]string, error) {
	defaults := p.data.GetDefaults()
	if defaults == nil {
		return [][]string{}, nil
	}

	budgetAmount, err := formatMinorUnits(defaults.GetBudgetAmount(), defaults.GetCurrency())
	if err != nil {
		// Legacy default settings may carry an unsupported currency. Render the
		// amount with the pre-multi-currency two-decimal behavior instead of
		// failing the whole data-rights export for a future-period default, and
		// count the fallback so unsupported legacy rows stay observable.
		exportmetrics.ExportCurrencyFormattingFallbackTotal.Inc()
		budgetAmount = formatMinorUnitsWithDigits(defaults.GetBudgetAmount(), 2)
	}

	row := []string{
		budgetAmount,
		strconv.FormatInt(int64(defaults.GetEssentialsPercent()), 10),
		strconv.FormatInt(int64(defaults.GetDesiresPercent()), 10),
		strconv.FormatInt(int64(defaults.GetSavingsPercent()), 10),
		defaults.GetCurrency(),
	}

	return [][]string{row}, nil
}
