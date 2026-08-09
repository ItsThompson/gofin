package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
)

// newReportingRouter builds the expense REST routes behind a middleware that puts
// a Sentry hub on the request context, which is what sentrygin does in production.
// A report that read anything other than the request context would find no hub
// here and record nothing.
//
// It also installs the log sink as slog.Default, because errkit writes its record
// through the package-level logger, which every service main sets to its own.
func newReportingRouter(t *testing.T, repo *mockExpenseRepository) (*gin.Engine, *errkittest.Transport, *bytes.Buffer) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	transport := &errkittest.Transport{}
	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(errkittest.ContextWithHub(c.Request.Context(), transport))
		c.Next()
	})
	NewRESTHandler(service.NewExpenseService(repo, time.Now, logger)).RegisterRoutes(engine)

	return engine, transport, buf
}

// A request-scoped 500 has exactly one owner. The service wrapper holds the error
// value, apierr.Respond must stay free of the SDK, and the gateway's request logger
// sees only a status code, so a single repository failure must cost a single event.
func TestREST_ARepositoryFailureYieldsExactlyOneEvent(t *testing.T) {
	repo := new(mockExpenseRepository)
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, errors.New("connection refused"))

	engine, transport, logs := newReportingRouter(t, repo)

	w := doJSONWithUserID(engine, http.MethodGet, "/api/expenses/exp-1", "user-1", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var body apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, apierr.CodeInternal, body.Code)
	assert.Equal(t, "An unexpected error occurred", body.Message,
		"the cause must not reach the client")

	events := transport.Events()
	require.Len(t, events, 1, "two events would mean a second reporter on the same failure")
	assert.Equal(t, "expense.get", events[0].Tags["operation"])
	assert.Equal(t, "expenses", events[0].Tags["domain"])
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
	assert.Equal(t, []string{"{{ default }}", "expense.get/internal"}, events[0].Fingerprint)
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, "connection refused")

	// The log record is the durable artifact: Sentry retains events for 30 days and
	// an incident may outlast that, so the migration must not have traded it away.
	records := errorRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "unexpected error", records[0]["msg"])
	assert.Contains(t, records[0]["error"], "connection refused")
}

// Every 4xx the services return is a typed *apierr.Error with an explicit Status,
// so the client-error class costs no quota by construction. There is no 422 in the
// codebase: apierr.Validation is a 400, and it is the highest-frequency failure
// category there is.
func TestREST_ClientErrorsYieldNoEvents(t *testing.T) {
	cases := []struct {
		name       string
		arrange    func(repo *mockExpenseRepository)
		target     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "service validation",
			arrange:    func(*mockExpenseRepository) {},
			target:     "/api/expenses?year=0&month=5",
			wantStatus: http.StatusBadRequest,
			wantCode:   apierr.CodeValidation,
		},
		{
			name: "not found",
			arrange: func(repo *mockExpenseRepository) {
				repo.On("GetExpenseByID", mock.Anything, "exp-999", "user-1").Return(nil, nil)
			},
			target:     "/api/expenses/exp-999",
			wantStatus: http.StatusNotFound,
			wantCode:   apierr.CodeNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			tc.arrange(repo)

			engine, transport, logs := newReportingRouter(t, repo)

			w := doJSONWithUserID(engine, http.MethodGet, tc.target, "user-1", nil)

			require.Equal(t, tc.wantStatus, w.Code)
			var body apierr.APIError
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Code)

			assert.Empty(t, transport.Events(), "a %d must not consume error quota", tc.wantStatus)
			assert.Empty(t, errorRecords(t, logs), "a client error is not an error-level defect")
		})
	}
}
