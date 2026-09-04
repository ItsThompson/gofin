package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/fx/internal/cache"
	"github.com/ItsThompson/gofin/services/fx/internal/model"
	sharedcurrency "github.com/ItsThompson/gofin/services/shared/currency"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

type fakeProvider struct {
	rates map[string]string
	err   error
	calls int
	now   time.Time
}

func (p *fakeProvider) FetchLatest(_ context.Context, expiresAt time.Time) (*model.ProviderSnapshot, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	rates := make(map[string]string, len(p.rates))
	for code, rate := range p.rates {
		rates[code] = rate
	}
	return &model.ProviderSnapshot{
		Source:        exchangesource.OpenExchangeRates,
		BaseCurrency:  model.BaseCurrencyUSD,
		RateTimestamp: "2026-08-15T10:00:00Z",
		CapturedAt:    p.now.UTC().Format(time.RFC3339),
		ExpiresAt:     expiresAt.UTC().Format(time.RFC3339),
		Rates:         rates,
	}, nil
}

func newTestConverter(provider *fakeProvider, now time.Time) *Converter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewConverter(provider, cache.NewRateCache(time.Hour), time.Hour, func() time.Time { return now }, logger)
}

func supportedRates(overrides map[string]string) map[string]string {
	rates := map[string]string{
		"USD": "1",
		"EUR": "0.8",
		"GBP": "0.5",
		"JPY": "100",
	}
	for _, definition := range sharedcurrency.All() {
		if _, ok := rates[definition.Code]; !ok {
			rates[definition.Code] = "1.25"
		}
	}
	for code, rate := range overrides {
		rates[code] = rate
	}
	return rates
}

func TestConvert_IdentityDoesNotCallProvider(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(nil), now: now}
	converter := newTestConverter(provider, now)

	response, err := converter.Convert(context.Background(), model.ConvertRequest{
		Amount:         1250,
		SourceCurrency: "USD",
		TargetCurrency: "USD",
		RequestedAt:    "2026-08-15T12:00:00Z",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1250), response.ConvertedAmount)
	assert.Equal(t, "1", response.ExchangeRate)
	assert.Equal(t, exchangesource.Identity, response.Source)
	assert.Equal(t, 0, provider.calls)
}

func TestConvert_UsesUSDBaseFormulaAndTargetMinorUnitRounding(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		amount         int64
		sourceCurrency string
		targetCurrency string
		rates          map[string]string
		wantAmount     int64
		wantRate       string
	}{
		{name: "EUR to USD", amount: 800, sourceCurrency: "EUR", targetCurrency: "USD", rates: supportedRates(nil), wantAmount: 1000, wantRate: "1.25"},
		{name: "USD to JPY", amount: 1250, sourceCurrency: "USD", targetCurrency: "JPY", rates: supportedRates(nil), wantAmount: 1250, wantRate: "100"},
		{name: "EUR to GBP", amount: 800, sourceCurrency: "EUR", targetCurrency: "GBP", rates: supportedRates(nil), wantAmount: 500, wantRate: "0.625"},
		{name: "rounds halves away from zero", amount: 5, sourceCurrency: "USD", targetCurrency: "JPY", rates: supportedRates(map[string]string{"JPY": "10"}), wantAmount: 1, wantRate: "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &fakeProvider{rates: tt.rates, now: now}
			converter := newTestConverter(provider, now)

			response, err := converter.Convert(context.Background(), model.ConvertRequest{
				Amount:         tt.amount,
				SourceCurrency: tt.sourceCurrency,
				TargetCurrency: tt.targetCurrency,
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantAmount, response.ConvertedAmount)
			assert.Equal(t, tt.wantRate, response.ExchangeRate)
			assert.Equal(t, model.CacheStatusMiss, response.CacheStatus)
		})
	}
}

func TestConvert_ValidatesCurrenciesAndRates(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		amount         int64
		sourceCurrency string
		targetCurrency string
		rates          map[string]string
		wantCode       model.ErrorCode
	}{
		{name: "invalid amount", amount: 0, sourceCurrency: "USD", targetCurrency: "EUR", rates: supportedRates(nil), wantCode: model.ErrorInvalidAmount},
		{name: "unsupported source", amount: 100, sourceCurrency: "DOGE", targetCurrency: "EUR", rates: supportedRates(nil), wantCode: model.ErrorUnsupportedCurrency},
		{name: "unsupported target", amount: 100, sourceCurrency: "USD", targetCurrency: "DOGE", rates: supportedRates(nil), wantCode: model.ErrorUnsupportedCurrency},
		{name: "missing source rate", amount: 100, sourceCurrency: "EUR", targetCurrency: "GBP", rates: map[string]string{"USD": "1", "GBP": "0.5"}, wantCode: model.ErrorRateMissing},
		{name: "missing target rate", amount: 100, sourceCurrency: "EUR", targetCurrency: "GBP", rates: map[string]string{"USD": "1", "EUR": "0.8"}, wantCode: model.ErrorRateMissing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &fakeProvider{rates: tt.rates, now: now}
			converter := newTestConverter(provider, now)

			_, err := converter.Convert(context.Background(), model.ConvertRequest{
				Amount:         tt.amount,
				SourceCurrency: tt.sourceCurrency,
				TargetCurrency: tt.targetCurrency,
			})

			var fxErr *model.Error
			require.ErrorAs(t, err, &fxErr)
			assert.Equal(t, tt.wantCode, fxErr.Code)
		})
	}
}

func TestConvert_UsesFreshCacheAndExpiresStaleEntries(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(nil), now: now}
	converter := newTestConverter(provider, now)

	_, err := converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "USD"})
	require.NoError(t, err)
	_, err = converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "GBP"})
	require.NoError(t, err)
	assert.Equal(t, 1, provider.calls)

	later := now.Add(time.Hour)
	converter.now = func() time.Time { return later }
	provider.now = later
	_, err = converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, 2, provider.calls)
}

func TestConvert_ProviderFailureWithFreshCacheUsesCache(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(nil), now: now}
	converter := newTestConverter(provider, now)

	_, err := converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "USD"})
	require.NoError(t, err)
	provider.err = errors.New("provider down")

	response, err := converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "GBP"})

	require.NoError(t, err)
	assert.Equal(t, int64(500), response.ConvertedAmount)
	assert.Equal(t, model.CacheStatusHit, response.CacheStatus)
	assert.Equal(t, 1, provider.calls)
}

func TestConvert_ProviderFailureWithoutCacheFails(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{err: errors.New("provider down"), now: now}
	converter := newTestConverter(provider, now)

	_, err := converter.Convert(context.Background(), model.ConvertRequest{Amount: 800, SourceCurrency: "EUR", TargetCurrency: "USD"})

	var fxErr *model.Error
	require.ErrorAs(t, err, &fxErr)
	assert.Equal(t, model.ErrorConversionUnavailable, fxErr.Code)
}

func TestCaptureSnapshot_ReturnsFullSupportedRateMap(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(nil), now: now}
	converter := newTestConverter(provider, now)

	response, err := converter.CaptureSnapshot(context.Background(), []string{"EUR", "GBP"})

	require.NoError(t, err)
	assert.Equal(t, model.CacheStatusMiss, response.CacheStatus)
	assert.Equal(t, int32(1), response.Snapshot.SnapshotVersion)
	assert.Equal(t, exchangesource.OpenExchangeRates, response.Snapshot.Source)
	assert.Equal(t, model.BaseCurrencyUSD, response.Snapshot.BaseCurrency)
	assert.NotEmpty(t, response.Snapshot.RateTimestamp)
	assert.NotEmpty(t, response.Snapshot.CapturedAt)
	assert.NotEmpty(t, response.Snapshot.ExpiresAt)
	for _, definition := range sharedcurrency.All() {
		assert.NotEmpty(t, response.Snapshot.RatesByCurrency[definition.Code], definition.Code)
	}
}

func TestConvertWithSnapshot_UsesProvidedSnapshotAndDoesNotCallProvider(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(map[string]string{"EUR": "9"}), now: now}
	converter := newTestConverter(provider, now)

	response, err := converter.ConvertWithSnapshot(model.ConvertWithSnapshotRequest{
		Amount:         800,
		SourceCurrency: "EUR",
		TargetCurrency: "GBP",
		Snapshot: model.CapturedRateSnapshot{
			SnapshotVersion: model.SnapshotVersion,
			Source:          exchangesource.OpenExchangeRates,
			BaseCurrency:    model.BaseCurrencyUSD,
			RateTimestamp:   "2026-08-15T10:00:00Z",
			CapturedAt:      "2026-08-15T12:00:00Z",
			ExpiresAt:       "2026-08-15T13:00:00Z",
			RatesByCurrency: supportedRates(nil),
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(500), response.ConvertedAmount)
	assert.Equal(t, model.CacheStatusProvided, response.CacheStatus)
	assert.Equal(t, 0, provider.calls)
}

func TestConvertWithSnapshot_RejectsInvalidSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{rates: supportedRates(nil), now: now}
	converter := newTestConverter(provider, now)

	_, err := converter.ConvertWithSnapshot(model.ConvertWithSnapshotRequest{
		Amount:         800,
		SourceCurrency: "EUR",
		TargetCurrency: "GBP",
		Snapshot:       model.CapturedRateSnapshot{SnapshotVersion: 2},
	})

	var fxErr *model.Error
	require.ErrorAs(t, err, &fxErr)
	assert.Equal(t, model.ErrorSnapshotIntegrityFailure, fxErr.Code)
}
