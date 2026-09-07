package deletion

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/getsentry/sentry-go"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// bindRecordingHub binds a client recording into the returned transport to the
// process-wide hub, and restores the previous client afterwards. The engine runs
// on pool contexts that carry no hub, so its reports reach Sentry through the
// hub-clone fallback, which is what this shape makes observable.
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

// A provider that exhausts its retries fails the job through the pool: the
// engine's own report is the failure's only Sentry record (the pool persists
// the reason but does not report), and it reaches Sentry through the
// background-hub fallback.
func TestEngine_RetryExhaustion_ReportsOneEvent(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	failProvider := &mockDeletionProvider{
		name:      "expense",
		failUntil: 3,
		err:       fmt.Errorf("persistent gRPC error"),
	}
	registry.Register(failProvider)

	transport := bindRecordingHub(t)

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-exhaust", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 10*time.Second, 10*time.Millisecond)

	events := transport.Events()
	require.Len(t, events, 1, "exactly one event for one failed job")
	assert.Equal(t, "deletion_job.run", events[0].Tags["operation"])
	assert.Equal(t, "datarights", events[0].Tags["domain"])
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value,
		"provider expense failed after 3 attempts")
	assert.Equal(t, "job-exhaust", events[0].Contexts["gofin"]["job_id"])
	assert.Equal(t, "user-1", events[0].Contexts["gofin"]["user_id"])
}
