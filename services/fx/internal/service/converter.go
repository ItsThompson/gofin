package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	sharedcurrency "github.com/ItsThompson/gofin/services/shared/currency"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"

	"github.com/ItsThompson/gofin/services/fx/internal/cache"
	fxmetrics "github.com/ItsThompson/gofin/services/fx/internal/metrics"
	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

type RateProvider interface {
	FetchLatest(ctx context.Context, expiresAt time.Time) (*model.ProviderSnapshot, error)
}

type Converter struct {
	provider RateProvider
	cache    *cache.RateCache
	maxAge   time.Duration
	now      func() time.Time
	logger   *slog.Logger
}

func NewConverter(provider RateProvider, cache *cache.RateCache, maxAge time.Duration, now func() time.Time, logger *slog.Logger) *Converter {
	return &Converter{provider: provider, cache: cache, maxAge: maxAge, now: now, logger: logger}
}

func (c *Converter) CaptureSnapshot(ctx context.Context, requiredCurrencies []string) (*model.SnapshotResult, error) {
	for _, code := range requiredCurrencies {
		if _, err := c.currencyDefinition(code, "requiredCurrencies"); err != nil {
			return nil, err
		}
	}
	snapshot, cacheStatus, err := c.providerSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	captured := c.toCapturedSnapshot(snapshot)
	if err := c.requireFullSupportedRates(captured.RatesByCurrency); err != nil {
		return nil, err
	}
	return &model.SnapshotResult{Snapshot: captured, CacheStatus: cacheStatus}, nil
}

func (c *Converter) Convert(ctx context.Context, request model.ConvertRequest) (*model.ConvertResponse, error) {
	start := time.Now()
	response, err := c.convert(ctx, request)
	c.recordConversion(request.SourceCurrency, request.TargetCurrency, start, err)
	return response, err
}

func (c *Converter) ConvertWithSnapshot(request model.ConvertWithSnapshotRequest) (*model.ConvertResponse, error) {
	start := time.Now()
	response, err := c.convertWithSnapshot(request)
	c.recordConversion(request.SourceCurrency, request.TargetCurrency, start, err)
	return response, err
}

func (c *Converter) convert(ctx context.Context, request model.ConvertRequest) (*model.ConvertResponse, error) {
	source, target, err := c.validateConversion(request.Amount, request.SourceCurrency, request.TargetCurrency)
	if err != nil {
		return nil, err
	}
	if request.SourceCurrency == request.TargetCurrency {
		return &model.ConvertResponse{
			ConvertedAmount: request.Amount,
			SourceCurrency:  request.SourceCurrency,
			TargetCurrency:  request.TargetCurrency,
			ExchangeRate:    "1",
			RateTimestamp:   fallbackTimestamp(request.RequestedAt, c.now()),
			Source:          exchangesource.Identity,
			CacheStatus:     model.CacheStatusHit,
		}, nil
	}
	snapshot, cacheStatus, err := c.providerSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return c.convertFromRates(request.Amount, source, target, snapshot.Rates, snapshot.RateTimestamp, snapshot.ExpiresAt, cacheStatus, exchangesource.OpenExchangeRates)
}

func (c *Converter) convertWithSnapshot(request model.ConvertWithSnapshotRequest) (*model.ConvertResponse, error) {
	source, target, err := c.validateConversion(request.Amount, request.SourceCurrency, request.TargetCurrency)
	if err != nil {
		return nil, err
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return nil, err
	}
	return c.convertFromRates(request.Amount, source, target, request.Snapshot.RatesByCurrency, request.Snapshot.RateTimestamp, request.Snapshot.ExpiresAt, model.CacheStatusProvided, request.Snapshot.Source)
}

func (c *Converter) validateConversion(amount int64, sourceCurrency string, targetCurrency string) (sharedcurrency.Definition, sharedcurrency.Definition, error) {
	if amount <= 0 {
		return sharedcurrency.Definition{}, sharedcurrency.Definition{}, model.NewError(model.ErrorInvalidAmount, "amount", fmt.Errorf("amount must be positive"))
	}
	source, err := c.currencyDefinition(sourceCurrency, "sourceCurrency")
	if err != nil {
		return sharedcurrency.Definition{}, sharedcurrency.Definition{}, err
	}
	target, err := c.currencyDefinition(targetCurrency, "targetCurrency")
	if err != nil {
		return sharedcurrency.Definition{}, sharedcurrency.Definition{}, err
	}
	return source, target, nil
}

func (c *Converter) currencyDefinition(code string, field string) (sharedcurrency.Definition, error) {
	definition, ok := sharedcurrency.Get(code)
	if !ok {
		return sharedcurrency.Definition{}, model.NewError(model.ErrorUnsupportedCurrency, field, fmt.Errorf("unsupported currency %q", code))
	}
	return definition, nil
}

func (c *Converter) providerSnapshot(ctx context.Context) (*model.ProviderSnapshot, string, error) {
	now := c.now().UTC()
	if snapshot, ok := c.cache.GetFresh(now); ok {
		fxmetrics.CacheHitsTotal.Inc()
		return snapshot, model.CacheStatusHit, nil
	}
	fxmetrics.CacheMissesTotal.Inc()
	expiresAt := now.Add(c.maxAge)
	snapshot, err := c.provider.FetchLatest(ctx, expiresAt)
	if err == nil {
		stored := c.cache.Store(*snapshot)
		return &stored, model.CacheStatusMiss, nil
	}
	if fresh, ok := c.cache.GetFresh(c.now().UTC()); ok {
		fxmetrics.CacheHitsTotal.Inc()
		return fresh, model.CacheStatusHit, nil
	}
	if _, ok := err.(*model.Error); ok {
		return nil, "", err
	}
	return nil, "", model.NewError(model.ErrorConversionUnavailable, "", err)
}

func (c *Converter) convertFromRates(
	amount int64,
	source sharedcurrency.Definition,
	target sharedcurrency.Definition,
	rates map[string]string,
	rateTimestamp string,
	expiresAt string,
	cacheStatus string,
	snapshotSource string,
) (*model.ConvertResponse, error) {
	sourceRate, err := rateForCurrency(rates, source.Code)
	if err != nil {
		return nil, err
	}
	targetRate, err := rateForCurrency(rates, target.Code)
	if err != nil {
		return nil, err
	}
	exchangeRate := new(big.Rat).Quo(targetRate, sourceRate)
	sourceMajor := minorToMajor(amount, source.MinorUnitDigits)
	targetMajor := new(big.Rat).Mul(sourceMajor, exchangeRate)
	convertedAmount := majorToRoundedMinor(targetMajor, target.MinorUnitDigits)
	return &model.ConvertResponse{
		ConvertedAmount: convertedAmount,
		SourceCurrency:  source.Code,
		TargetCurrency:  target.Code,
		ExchangeRate:    decimalString(exchangeRate),
		RateTimestamp:   rateTimestamp,
		Source:          snapshotSource,
		CacheStatus:     cacheStatus,
		ExpiresAt:       expiresAt,
	}, nil
}

func rateForCurrency(rates map[string]string, code string) (*big.Rat, error) {
	rateValue, ok := rates[code]
	if !ok || rateValue == "" {
		return nil, model.NewError(model.ErrorRateMissing, code, fmt.Errorf("missing rate for %s", code))
	}
	rate, err := parsePositiveRat(rateValue)
	if err != nil {
		return nil, model.NewError(model.ErrorRateMissing, code, err)
	}
	return rate, nil
}

func (c *Converter) toCapturedSnapshot(snapshot *model.ProviderSnapshot) model.CapturedRateSnapshot {
	rates := make(map[string]string, len(snapshot.Rates))
	for code, rate := range snapshot.Rates {
		if sharedcurrency.IsSupported(code) {
			rates[code] = rate
		}
	}
	return model.CapturedRateSnapshot{
		SnapshotVersion: model.SnapshotVersion,
		Source:          snapshot.Source,
		BaseCurrency:    snapshot.BaseCurrency,
		RateTimestamp:   snapshot.RateTimestamp,
		CapturedAt:      snapshot.CapturedAt,
		ExpiresAt:       snapshot.ExpiresAt,
		RatesByCurrency: rates,
	}
}

func (c *Converter) requireFullSupportedRates(rates map[string]string) error {
	for _, definition := range sharedcurrency.All() {
		if _, err := rateForCurrency(rates, definition.Code); err != nil {
			return err
		}
	}
	return nil
}

func validateSnapshot(snapshot model.CapturedRateSnapshot) error {
	if snapshot.SnapshotVersion != model.SnapshotVersion || snapshot.Source != exchangesource.OpenExchangeRates || snapshot.BaseCurrency != model.BaseCurrencyUSD {
		return model.NewError(model.ErrorSnapshotIntegrityFailure, "snapshot", fmt.Errorf("invalid snapshot metadata"))
	}
	if snapshot.RateTimestamp == "" || snapshot.CapturedAt == "" || snapshot.ExpiresAt == "" || len(snapshot.RatesByCurrency) == 0 {
		return model.NewError(model.ErrorSnapshotIntegrityFailure, "snapshot", fmt.Errorf("snapshot missing required fields"))
	}
	return nil
}

func fallbackTimestamp(requestedAt string, now time.Time) string {
	if requestedAt != "" {
		return requestedAt
	}
	return now.UTC().Format(time.RFC3339)
}

func (c *Converter) recordConversion(sourceCurrency string, targetCurrency string, start time.Time, err error) {
	result := "success"
	if err != nil {
		result = "failure"
	}
	fxmetrics.ConversionRequestsTotal.WithLabelValues(sourceCurrency, targetCurrency, result).Inc()
	fxmetrics.ConversionLatencySeconds.WithLabelValues(sourceCurrency, targetCurrency).Observe(time.Since(start).Seconds())
	if err != nil {
		c.logger.Warn("fx conversion failed", slog.String("source_currency", sourceCurrency), slog.String("target_currency", targetCurrency), slog.String("error", err.Error()))
		return
	}
	c.logger.Info("fx conversion completed", slog.String("source_currency", sourceCurrency), slog.String("target_currency", targetCurrency))
}
