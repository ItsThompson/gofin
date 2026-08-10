package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// bindRecordingHub binds a recording client to the process-wide hub, which is
// where an export job's report lands: a job has no request context, so errkit
// falls back to a clone of the global hub. Tests using it must not run in
// parallel.
func bindRecordingHub(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))

	return transport
}

// The export failure path has two audiences and they get different text. The user
// sees the reason persisted through FailJob, which must stay free of transport
// detail; the operator gets the event, which is worth nothing without it.
func TestEngine_ProviderFailure_ReportsTheCauseTheReasonWithholds(t *testing.T) {
	repo := &mockRepo{}
	logger, _ := newReportingLogger(t)
	transport := bindRecordingHub(t)
	cause := "dial tcp 10.0.0.7:5432: connect: connection refused"

	eng := NewEngine(staticProviders(&stubProvider{
		name: "profile",
		err:  fmt.Errorf("querying profile: %s", cause),
	}), nopFinance{}, repo, newMockSender(), 5, 5*time.Minute, logger)
	eng.Submit("job-report", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	require.Equal(t, "Failed to collect profile data", failed[0].ErrMsg)
	require.NotContains(t, failed[0].ErrMsg, cause)

	events := transport.Events()
	require.Len(t, events, 1, "one failed job is one event")

	exception := events[0].Exception[len(events[0].Exception)-1]
	assert.Contains(t, exception.Value, cause, "the event is the only place the cause survives")
	assert.Equal(t, "export_job.run", events[0].Tags["operation"])
	assert.Equal(t, "datarights", events[0].Tags["domain"])
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
	assert.Equal(t, []string{"{{ default }}", "export_job.run/internal"}, events[0].Fingerprint)

	// Exact equality, not key presence: it is what proves nothing else rides along.
	// The job holds the user's email address and every figure in the export, and
	// none of it belongs on an event.
	assert.Equal(t, map[string]any{
		"job_id":  "job-report",
		"user_id": "user-1",
		"stage":   "collection",
	}, events[0].Contexts["gofin"])
}

// A successful export must cost nothing. Exports are user-triggered, so a report
// on the success path would scale with use rather than with defects.
func TestEngine_SuccessfulExport_ReportsNothing(t *testing.T) {
	repo := &mockRepo{}
	logger, _ := newReportingLogger(t)
	transport := bindRecordingHub(t)

	eng := NewEngine(staticProviders(&stubProvider{
		name:    "profile",
		headers: []string{"username"},
		rows:    [][]string{{"alex"}},
	}), nopFinance{}, repo, newMockSender(), 5, 5*time.Minute, logger)
	eng.Submit("job-ok", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.Empty(t, transport.Events())
}

// recordFailure must report on the context the job passed it. A background context
// would still reach Sentry, through a clone of the global hub, so no field on the
// event would look wrong and the downgrade would be silent. Which hub receives it
// is the only observable difference, so that is what this pins.
func TestRecordFailure_ReportsOnTheContextItIsGiven(t *testing.T) {
	logger, _ := newReportingLogger(t)
	global := bindRecordingHub(t)
	scoped := &errkittest.Transport{}

	eng := NewEngine(staticProviders(&stubProvider{name: "profile"}),
		nopFinance{}, &mockRepo{}, newMockSender(), 5, 5*time.Minute, logger)

	err := eng.recordFailure(errkittest.ContextWithHub(context.Background(), scoped),
		"job-ctx", "user-1", "Export timed out", errors.New("context deadline exceeded"),
		"finance_fetch", time.Now())

	require.Error(t, err)
	assert.Len(t, scoped.Events(), 1, "the report must use the context the job passed")
	assert.Empty(t, global.Events(), "a background context would land here instead")
}
