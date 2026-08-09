package httpx_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/httpx"
)

// restMeta is the shape a migrated REST handler passes: its own operation and
// domain, with the kind left to default.
var restMeta = errkit.Meta{
	Kind:   errkit.KindDatabase,
	Op:     "expense.create",
	Domain: "expenses",
	Msg:    "unexpected error",
}

// newReportingContext returns a gin context whose request carries a Sentry hub,
// which is the shape sentrygin produces: the hub reaches a handler through the
// request's context, not through the gin context. A report that read
// context.Background() instead would find no hub here and record nothing.
func newReportingContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *errkittest.Transport) {
	t.Helper()

	transport := &errkittest.Transport{}
	req := httptest.NewRequest(http.MethodPost, "/api/expenses", nil)
	c, w := newContextWithRequest(req.WithContext(errkittest.ContextWithHub(req.Context(), transport)))
	return c, w, transport
}

// A request-scoped 500 must cost exactly one event: the service wrapper is the
// single owner, and neither apierr.Respond nor the gateway's request logger adds
// one.
func TestRespondError_AnUnclassifiedErrorYieldsExactlyOneEvent(t *testing.T) {
	c, w, transport := newReportingContext(t)

	httpx.RespondError(c, fmt.Errorf("insert expense: %w", errors.New("connection refused")), restMeta)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "database", events[0].Tags["error_kind"])
	assert.Equal(t, "expense.create", events[0].Tags["operation"])
	assert.Equal(t, "expenses", events[0].Tags["domain"])
	assert.Equal(t, []string{"{{ default }}", "expense.create/database"}, events[0].Fingerprint)
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, "connection refused")
}

// Every 4xx in the codebase is a typed *apierr.Error carrying an explicit Status,
// so the classified branch excludes the whole client-error class by construction.
// Validation failures are the highest-frequency category in the codebase, and the
// allowance is 5,000 events a month shared org-wide.
func TestRespondError_AClassifiedErrorIsNeverReported(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "validation",
			err:        apierr.Validation("Invalid request body", map[string]string{"Amount": "required"}),
			wantStatus: http.StatusBadRequest,
			wantCode:   apierr.CodeValidation,
		},
		{
			name:       "not found",
			err:        apierr.NotFound("Expense not found"),
			wantStatus: http.StatusNotFound,
			wantCode:   apierr.CodeNotFound,
		},
		{
			name:       "wrapped not found",
			err:        fmt.Errorf("loading expense: %w", apierr.NotFound("Expense not found")),
			wantStatus: http.StatusNotFound,
			wantCode:   apierr.CodeNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w, transport := newReportingContext(t)

			httpx.RespondError(c, tc.err, restMeta)

			require.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantCode, decodeBody(t, w)["code"])
			assert.Empty(t, transport.Events(), "a %d must not consume error quota", tc.wantStatus)
		})
	}
}

// The wire contract belongs to apierr.Respond alone. Reporting is additive, so
// every response byte must match what the same error produced before the helper
// existed.
func TestRespondError_WritesExactlyWhatApierrRespondWould(t *testing.T) {
	for _, err := range []error{
		errors.New("connection refused"),
		apierr.Validation("Invalid request body", map[string]string{"Amount": "required"}),
		apierr.NotFound("Expense not found"),
		&apierr.Error{Code: "NO_STATUS", Message: "status unset"},
	} {
		reported, reportedRecorder, _ := newReportingContext(t)
		httpx.RespondError(reported, err, restMeta)

		direct, directRecorder := newContextWithRequest(httptest.NewRequest(http.MethodPost, "/api/expenses", nil))
		apierr.Respond(direct, err)

		assert.Equal(t, directRecorder.Code, reportedRecorder.Code, err)
		assert.JSONEq(t, directRecorder.Body.String(), reportedRecorder.Body.String(), err)
	}
}
