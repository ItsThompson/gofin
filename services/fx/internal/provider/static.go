package provider

import (
	"context"
	"time"

	sharedcurrency "github.com/ItsThompson/gofin/services/shared/currency"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

// StaticProvider serves a fixed rate of 1 for every supported currency. It is
// used only for local development and CI, where no Open Exchange Rates app ID
// is configured, so FX-dependent flows still work with deterministic values.
type StaticProvider struct {
	now func() time.Time
}

func NewStaticProvider(now func() time.Time) *StaticProvider {
	return &StaticProvider{now: now}
}

func (p *StaticProvider) FetchLatest(_ context.Context, expiresAt time.Time) (*model.ProviderSnapshot, error) {
	rates := make(map[string]string, len(sharedcurrency.All())+1)
	rates[model.BaseCurrencyUSD] = "1"
	for _, definition := range sharedcurrency.All() {
		rates[definition.Code] = "1"
	}

	now := p.now().UTC()
	return &model.ProviderSnapshot{
		Source:        model.SourceOpenExchangeRates,
		BaseCurrency:  model.BaseCurrencyUSD,
		RateTimestamp: now.Format(time.RFC3339),
		CapturedAt:    now.Format(time.RFC3339),
		ExpiresAt:     expiresAt.UTC().Format(time.RFC3339),
		Rates:         rates,
	}, nil
}
