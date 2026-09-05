package providers

import (
	"fmt"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// BuildPeriodCurrencyMap derives a "year:month" -> reporting currency lookup
// from the shared per-job finance response. The expenses provider uses it to
// normalize legacy migration rows to each period's immutable reporting
// currency, without calling finance again during collection.
func BuildPeriodCurrencyMap(data *financepb.AllUserDataResponse) map[string]string {
	periods := data.GetPeriods()
	periodCurrencies := make(map[string]string, len(periods))
	for _, period := range periods {
		periodCurrencies[periodCurrencyKey(period.GetYear(), period.GetMonth())] = period.GetReportingCurrencyCode()
	}
	return periodCurrencies
}

// periodCurrencyKey is the canonical key format shared by the expense service's
// own period-context cache, keeping the two services' lookups aligned.
func periodCurrencyKey(year, month int32) string {
	return fmt.Sprintf("%d:%d", year, month)
}
