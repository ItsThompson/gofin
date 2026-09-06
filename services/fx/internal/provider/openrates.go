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
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

// reportWindow bounds how often one provider failure class is reported. The
// converter calls FetchLatest on cache misses, so an outage fails every
// conversion request; a plain provider report would emit thousands of events
// an hour against the organization's shared monthly allowance. One report per
// window per failure class keeps one stable issue per outage while every failed
// call still writes its own site record.
//
// It is a variable rather than a constant because it is the test seam: an
// internal test shrinks it before constructing the provider (NewLimiter panics
// on non-positive windows, and Limiter.now is not publicly injectable).
var reportWindow = time.Hour

type OpenRatesProvider struct {
	client     *http.Client
	baseURL    string
	appID      string
	retryCount int
	now        func() time.Time
	logger     *slog.Logger

	// One Limiter per failure class: the window belongs to the site it guards,
	// so a shared Limiter would suppress the other class's first report.
	netReports  *errkit.Limiter
	authReports *errkit.Limiter
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
		netReports:  errkit.NewLimiter(reportWindow),
		authReports: errkit.NewLimiter(reportWindow),
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

	// The terminal exit, once per user-visible call rather than per attempt:
	// network errors and 5xx responses both terminate here as the same
	// ErrorConversionUnavailable, so retry attempts cannot multiply records.
	// The site record is unconditional (it is the per-occurrence artifact that
	// shows an operator how much traffic the outage affects); the report is
	// gated to one per window per failure class.
	p.logger.Error("fx provider fetch failed",
		slog.String("endpoint", p.baseURL),
		slog.Int("attempts", p.retryCount+1),
		slog.String("error", lastErr.Error()),
	)
	if p.netReports.Allow() {
		// One issue for the whole class: the stack varies with the call while
		// the meaning (the provider is unreachable) does not. endpoint is the
		// configured base URL, never the full request URL, because the query
		// carries the API key.
		_ = errkit.Report(ctx, lastErr, errkit.Meta{
			Kind:       errkit.KindUpstream,
			Op:         "fx.provider_fetch",
			Domain:     "fx",
			Msg:        "fx provider fetch failed",
			GroupKey:   "fx.provider_unreachable",
			GroupExact: true,
			Data: map[string]any{
				"endpoint": p.baseURL,
			},
		})
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
		return nil, model.NewError(model.ErrorConversionUnavailable, "", err)
	}
	defer func() { _ = response.Body.Close() }()

	statusCode := strconv.Itoa(response.StatusCode)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		fxmetrics.ProviderRequestsTotal.WithLabelValues("auth_failed", statusCode).Inc()
		fxmetrics.ProviderLatencySeconds.WithLabelValues("auth_failed").Observe(time.Since(start).Seconds())
		authErr := model.NewError(model.ErrorProviderAuthFailed, "", fmt.Errorf("provider returned %d", response.StatusCode))
		// The record stays per occurrence; the report is bounded to one per
		// window so a long-lived bad credential cannot burn the allowance.
		p.logger.Error("fx provider authentication failed", slog.Int("status", response.StatusCode))
		if p.authReports.Allow() {
			_ = errkit.Report(ctx, authErr, errkit.Meta{
				Kind:       errkit.KindUpstream,
				Op:         "fx.provider_auth",
				Domain:     "fx",
				Msg:        "fx provider authentication failed",
				GroupKey:   "fx.provider_auth_failed",
				GroupExact: true,
				Data: map[string]any{
					"status": response.StatusCode,
				},
			})
		}
		return nil, authErr
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
