package provider

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

func TestOpenRatesProvider_FetchLatestParsesUSDSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "app-id", request.URL.Query().Get("app_id"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"timestamp":1786797600,"base":"USD","rates":{"EUR":0.8,"GBP":0.5}}`))
	}))
	t.Cleanup(server.Close)
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	provider := NewOpenRatesProvider(server.Client(), server.URL, "app-id", 0, func() time.Time { return now }, silentLogger())

	snapshot, err := provider.FetchLatest(context.Background(), now.Add(time.Hour))

	require.NoError(t, err)
	assert.Equal(t, model.SourceOpenExchangeRates, snapshot.Source)
	assert.Equal(t, model.BaseCurrencyUSD, snapshot.BaseCurrency)
	assert.Equal(t, "2026-08-15T12:00:00Z", snapshot.CapturedAt)
	assert.Equal(t, "2026-08-15T13:00:00Z", snapshot.ExpiresAt)
	assert.Equal(t, "1", snapshot.Rates["USD"])
	assert.Equal(t, "0.8", snapshot.Rates["EUR"])
}

func TestOpenRatesProvider_MapsProviderFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   model.ErrorCode
	}{
		{name: "auth failure", statusCode: http.StatusUnauthorized, body: `{}`, wantCode: model.ErrorProviderAuthFailed},
		{name: "invalid response", statusCode: http.StatusOK, body: `{"timestamp":0,"base":"USD","rates":{}}`, wantCode: model.ErrorProviderResponseInvalid},
		{name: "provider unavailable", statusCode: http.StatusInternalServerError, body: `{}`, wantCode: model.ErrorConversionUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)
			now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
			provider := NewOpenRatesProvider(server.Client(), server.URL, "app-id", 0, func() time.Time { return now }, silentLogger())

			_, err := provider.FetchLatest(context.Background(), now.Add(time.Hour))

			var fxErr *model.Error
			require.ErrorAs(t, err, &fxErr)
			assert.Equal(t, tt.wantCode, fxErr.Code)
		})
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
