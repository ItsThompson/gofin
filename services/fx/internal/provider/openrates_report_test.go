package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/fx/internal/model"
)

// roundTripperFunc adapts a function to http.RoundTripper, so a stub client can
// fail the transport deterministically without a network.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newReportingProvider returns a provider whose records (its own site records
// and errkit's report records, which go through slog.Default) all land in the
// returned buffer.
func newReportingProvider(t *testing.T, retryCount int, transport http.RoundTripper) (*OpenRatesProvider, *bytes.Buffer) {
	t.Helper()

	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	client := &http.Client{Transport: transport}
	return NewOpenRatesProvider(client, "https://open.er-api.io/v6/latest", "app-id",
		retryCount, func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC) }, logger), buf
}

// shrinkReportWindow installs a test-sized limiter window and restores the
// production one afterwards.
func shrinkReportWindow(t *testing.T, window time.Duration) {
	t.Helper()

	previous := reportWindow
	reportWindow = window
	t.Cleanup(func() { reportWindow = previous })
}

func errorLevelRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["level"] == slog.LevelError.String() {
			records = append(records, record)
		}
	}
	return records
}

var networkFailure = errors.New("dial tcp: connection refused")

// The provider is the only reporter for the swallowed conversion surface, and
// an outage fails every conversion request, so the network class must report
// once per window while every failed call still writes its site record. After
// two failing calls: one event (grouped exactly, so every outage is one issue)
// and three error records (two site records plus the first call's report).
func TestFetchLatest_NetworkFailure_ReportsOncePerWindow(t *testing.T) {
	shrinkReportWindow(t, time.Second)
	provider, logs := newReportingProvider(t, 0, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, networkFailure
	}))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	expiresAt := time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC)
	_, err := provider.FetchLatest(ctx, expiresAt)
	require.Error(t, err)

	events := transport.Events()
	require.Len(t, events, 1, "the first failure in a window reports")
	assert.Equal(t, "fx.provider_fetch", events[0].Tags["operation"])
	assert.Equal(t, "fx", events[0].Tags["domain"])
	assert.Equal(t, "upstream", events[0].Tags["error_kind"])
	assert.Equal(t, []string{"fx.provider_unreachable"}, events[0].Fingerprint, "GroupExact collapses every outage into one issue")
	assert.Equal(t, map[string]any{"endpoint": "https://open.er-api.io/v6/latest"}, events[0].Contexts["gofin"])
	assert.Len(t, errorLevelRecords(t, logs), 2, "first call: one site record plus the report's own record")

	_, err = provider.FetchLatest(ctx, expiresAt)
	require.Error(t, err)

	events = transport.Events()
	require.Len(t, events, 1, "the second failure in the window reports nothing")
	assert.Len(t, errorLevelRecords(t, logs), 3, "but it still writes its site record")
}

// The site record is pinned at the terminal exit, once per FetchLatest call, so
// retry attempts cannot multiply records.
func TestFetchLatest_RetriesWriteOneSiteRecord(t *testing.T) {
	shrinkReportWindow(t, 10*time.Second)
	provider, logs := newReportingProvider(t, 2, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, networkFailure
	}))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := provider.FetchLatest(ctx, time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC))
	require.Error(t, err)

	var fxErr *model.Error
	require.ErrorAs(t, err, &fxErr)
	assert.Equal(t, model.ErrorConversionUnavailable, fxErr.Code)

	require.Len(t, transport.Events(), 1)
	records := errorLevelRecords(t, logs)
	require.Len(t, records, 2, "one site record plus the report's record, not one per attempt")
	assert.Equal(t, "fx provider fetch failed", records[0]["msg"])
	assert.Equal(t, float64(3), records[0]["attempts"])
}

// The two failure classes own separate windows: a consumed network limiter must
// not suppress the first auth report.
func TestFetchLatest_AuthFailure_ReportsIndependentlyOfTheNetworkLimiter(t *testing.T) {
	shrinkReportWindow(t, time.Second)

	// A transport that fails the network first, then serves 401.
	calls := 0
	provider, logs := newReportingProvider(t, 0, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, networkFailure
		}
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	}))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	expiresAt := time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC)
	_, err := provider.FetchLatest(ctx, expiresAt)
	require.Error(t, err)

	_, err = provider.FetchLatest(ctx, expiresAt)
	require.Error(t, err)
	var fxErr *model.Error
	require.ErrorAs(t, err, &fxErr)
	assert.Equal(t, model.ErrorProviderAuthFailed, fxErr.Code)

	events := transport.Events()
	require.Len(t, events, 2, "the auth report fires although the network limiter was just consumed")
	assert.Equal(t, []string{"fx.provider_unreachable"}, events[0].Fingerprint)
	assert.Equal(t, []string{"fx.provider_auth_failed"}, events[1].Fingerprint)
	assert.Equal(t, "fx.provider_auth", events[1].Tags["operation"])
	assert.Equal(t, 401, events[1].Contexts["gofin"]["status"])

	records := errorLevelRecords(t, logs)
	require.Len(t, records, 4, "per call: site record plus report record, auth included")
	assert.Equal(t, "fx provider authentication failed", records[2]["msg"])
}
