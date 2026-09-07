package jobrunner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/getsentry/sentry-go"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// bindRecordingHub binds a client recording into the returned transport to the
// process-wide hub, and restores the previous client afterwards. The pool runs
// on contexts that carry no hub, so its reports reach Sentry through the
// hub-clone fallback of sentry.CurrentHub(), which is what this shape makes
// observable (same precedent as serverkit's sentryevent tests).
//
// Tests using it must not run in parallel.
func bindRecordingHub(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))

	return transport
}

// A FailJob store failure is a background persistence fault nothing else can
// report: the caller (Submit) is fire-and-forget and the job already failed.
// The report must reach Sentry through the hub-clone fallback even though the
// pool hands it a background context.
func TestPool_FailJobStoreFailure_ReportsThroughBackgroundHubFallback(t *testing.T) {
	store := &fakeStore{failErr: errors.New("deletion write: connection refused")}
	execute := func(_ context.Context, _, _ string) error {
		return errors.New("strategy blew up")
	}

	transport := bindRecordingHub(t)

	pool := New(5, time.Minute, store, execute, testLogger())
	pool.Submit("job-err", "user-1")

	require.Eventually(t, func() bool {
		return len(store.failedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	events := transport.Events()
	require.Len(t, events, 1, "exactly one event for the status-write failure")
	assert.Equal(t, "jobrunner.status_write", events[0].Tags["operation"])
	assert.Equal(t, "datarights", events[0].Tags["domain"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value,
		"connection refused")
	assert.Equal(t, "job-err", events[0].Contexts["gofin"]["job_id"])
	assert.Equal(t, "user-1", events[0].Contexts["gofin"]["user_id"])

	// The pool itself still returns normally: the failure is recorded, not raised.
	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 0
	}, 2*time.Second, 10*time.Millisecond)
}
