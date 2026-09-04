package provider

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedcurrency "github.com/ItsThompson/gofin/services/shared/currency"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

func TestStaticProviderFetchLatestCoversSupportedCurrencies(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	provider := NewStaticProvider(func() time.Time { return now })

	snapshot, err := provider.FetchLatest(context.Background(), now.Add(time.Hour))

	require.NoError(t, err)
	assert.Equal(t, exchangesource.OpenExchangeRates, snapshot.Source)
	assert.Equal(t, model.BaseCurrencyUSD, snapshot.BaseCurrency)
	assert.NotEmpty(t, snapshot.RateTimestamp)
	assert.NotEmpty(t, snapshot.CapturedAt)
	assert.NotEmpty(t, snapshot.ExpiresAt)
	for _, definition := range sharedcurrency.All() {
		assert.Equal(t, "1", snapshot.Rates[definition.Code], "missing static rate for %s", definition.Code)
	}
}
