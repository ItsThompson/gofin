package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	fxmetrics "github.com/ItsThompson/gofin/services/fx/internal/metrics"
	"github.com/ItsThompson/gofin/services/fx/internal/model"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

type OpenRatesProvider struct {
	client     *http.Client
	baseURL    string
	appID      string
	retryCount int
	now        func() time.Time
	logger     *slog.Logger
}

type openRatesResponse struct {
	Timestamp int64                      `json:"timestamp"`
	Base      string                     `json:"base"`
	Rates     map[string]json.RawMessage `json:"rates"`
}

func NewOpenRatesProvider(
	client *http.Client,
	baseURL string,
	appID string,
	retryCount int,
	now func() time.Time,
	logger *slog.Logger,
) *OpenRatesProvider {
	return &OpenRatesProvider{
		client:     client,
		baseURL:    baseURL,
		appID:      appID,
		retryCount: retryCount,
		now:        now,
		logger:     logger,
	}
}

func (p *OpenRatesProvider) FetchLatest(ctx context.Context, expiresAt time.Time) (*model.ProviderSnapshot, error) {
	var lastErr error
	for attempt := 0; attempt <= p.retryCount; attempt++ {
		if attempt > 0 {
			p.waitBeforeRetry(ctx, attempt)
		}
		snapshot, err := p.fetchOnce(ctx, expiresAt)
		if err == nil {
			return snapshot, nil
		}
		lastErr = err
		if fxErr, ok := err.(*model.Error); ok && fxErr.Code != model.ErrorConversionUnavailable {
			return nil, err
		}
	}
	return nil, lastErr
}

func (p *OpenRatesProvider) fetchOnce(ctx context.Context, expiresAt time.Time) (*model.ProviderSnapshot, error) {
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", fmt.Errorf("invalid provider URL: %w", err))
	}
	query := endpoint.Query()
	query.Set("app_id", p.appID)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", err)
	}

	start := time.Now()
	response, err := p.client.Do(request)
	if err != nil {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("error", "network").Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("error").Observe(time.Since(start).Seconds())
		p.logger.Warn("fx provider request failed", slog.String("error", err.Error()))
		return nil, model.NewError(model.ErrorConversionUnavailable, "", err)
	}
	defer func() { _ = response.Body.Close() }()

	statusCode := strconv.Itoa(response.StatusCode)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("auth_failed", statusCode).Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("auth_failed").Observe(time.Since(start).Seconds())
		p.logger.Error("fx provider authentication failed", slog.Int("status", response.StatusCode))
		return nil, model.NewError(model.ErrorProviderAuthFailed, "", fmt.Errorf("provider returned %d", response.StatusCode))
	}
	if response.StatusCode >= 500 {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("retryable_error", statusCode).Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("retryable_error").Observe(time.Since(start).Seconds())
		return nil, model.NewError(model.ErrorConversionUnavailable, "", fmt.Errorf("provider returned %d", response.StatusCode))
	}
	if response.StatusCode != http.StatusOK {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("invalid", statusCode).Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("invalid").Observe(time.Since(start).Seconds())
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", fmt.Errorf("provider returned %d", response.StatusCode))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", err)
	}
	snapshot, err := decodeProviderResponse(body, p.now(), expiresAt)
	if err != nil {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("invalid", statusCode).Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("invalid").Observe(time.Since(start).Seconds())
		return nil, err
	}
	fxmetrics.ProviderRequestsTotal.WithLabelValues("success", statusCode).Inc()
	fxmetrics.ProviderLatencySeconds.WithLabelValues("success").Observe(time.Since(start).Seconds())
	return snapshot, nil
}

func (p *OpenRatesProvider) waitBeforeRetry(ctx context.Context, attempt int) {
	delay := time.Duration(attempt*attempt) * 100 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func decodeProviderResponse(body []byte, capturedAt time.Time, expiresAt time.Time) (*model.ProviderSnapshot, error) {
	var payload openRatesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", err)
	}
	if payload.Timestamp <= 0 || payload.Base != model.BaseCurrencyUSD || len(payload.Rates) == 0 {
		return nil, model.NewError(model.ErrorProviderResponseInvalid, "", fmt.Errorf("provider response missing required fields"))
	}
	rates := make(map[string]string, len(payload.Rates)+1)
	rates[model.BaseCurrencyUSD] = "1"
	for code, raw := range payload.Rates {
		rate, err := rateToString(raw)
		if err != nil {
			return nil, model.NewError(model.ErrorProviderResponseInvalid, "", fmt.Errorf("invalid rate for %s: %w", code, err))
		}
		rates[code] = rate
	}
	return &model.ProviderSnapshot{
		Source:        exchangesource.OpenExchangeRates,
		BaseCurrency:  payload.Base,
		RateTimestamp: time.Unix(payload.Timestamp, 0).UTC().Format(time.RFC3339),
		CapturedAt:    capturedAt.UTC().Format(time.RFC3339),
		ExpiresAt:     expiresAt.UTC().Format(time.RFC3339),
		Rates:         rates,
	}, nil
}

func rateToString(raw json.RawMessage) (string, error) {
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return "", err
	}
	rat := newRat(number.String())
	if rat == nil || rat.Sign() <= 0 {
		return "", fmt.Errorf("rate must be positive")
	}
	return number.String(), nil
}

func newRat(value string) *big.Rat {
	rat := new(big.Rat)
	if _, ok := rat.SetString(value); !ok {
		return nil
	}
	return rat
}
