package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/gateway/internal/proxy"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// This site is the largest quota exposure in the backend: it fires once per
// proxied request for as long as a downstream is down, and a single open browser
// tab keeps requesting. Unbounded, one outage spends a monthly allowance that is
// shared across the whole organization, on an incident Prometheus already pages
// for.
//
// It is also the representative for the sink pattern. The immudb reconnection sink
// is the same shape: a per-occurrence record, and a report behind the same limiter.
func TestNewServiceProxy_ReportsAnUnreachableTargetOncePerWindow(t *testing.T) {
	const requests = 5

	// Nothing is listening, so every request fails in the dialer.
	target, err := url.Parse("http://127.0.0.1:19999")
	require.NoError(t, err)

	logger, sink := serverkittest.NewLogger()
	handler := proxy.NewServiceProxy(target, logger)

	transport := &errkittest.Transport{}
	for range requests {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req.WithContext(errkittest.ContextWithHub(req.Context(), transport)))
		require.Equal(t, http.StatusBadGateway, rec.Code)
	}

	// The record is not rate limited. It is the durable artifact, it is what shows
	// how much traffic the outage affects, and log volume is not the constraint.
	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	assert.Len(t, records, requests)

	events := transport.Events()
	require.Len(t, events, 1, "%d requests during one outage must not be %d events", requests, requests)
	assert.Equal(t, "upstream", events[0].Tags["error_kind"])
	assert.Equal(t, "gateway.proxy", events[0].Tags["operation"])
	assert.Equal(t, "platform", events[0].Tags["domain"])

	// GroupExact, so a multi-target outage is one issue rather than one per target,
	// and the target itself is a context detail. A raw URL is never a tag.
	assert.Equal(t, []string{"gateway.downstream_unreachable"}, events[0].Fingerprint)
	assert.NotContains(t, events[0].Tags, "target")

	gofinContext, ok := events[0].Contexts["gofin"]
	require.True(t, ok)
	assert.Equal(t, "http://127.0.0.1:19999", gofinContext["target"])
	assert.Equal(t, "/api/test", gofinContext["path"])
}

// Each proxy holds its own limiter and the router builds one per downstream, so a
// first outage cannot silence the report of a second one.
func TestNewServiceProxy_EachTargetHasItsOwnWindow(t *testing.T) {
	transport := &errkittest.Transport{}

	for _, address := range []string{"http://127.0.0.1:19998", "http://127.0.0.1:19999"} {
		target, err := url.Parse(address)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		proxy.NewServiceProxy(target, newSilentLogger()).
			ServeHTTP(httptest.NewRecorder(), req.WithContext(errkittest.ContextWithHub(req.Context(), transport)))
	}

	assert.Len(t, transport.Events(), 2)
}
