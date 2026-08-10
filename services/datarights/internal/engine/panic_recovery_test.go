package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panickingProvider panics inside Collect, which the export engine runs on an
// errgroup goroutine. recover() does not cross goroutines and errgroup
// deliberately does not recover, so without the engine's own guard this test
// takes the whole test binary down with it.
type panickingProvider struct {
	name string
}

func (p *panickingProvider) Name() string      { return p.name }
func (p *panickingProvider) Headers() []string { return []string{"col"} }
func (p *panickingProvider) Collect(context.Context, string) ([][]string, error) {
	panic("provider exploded holding user@example.com")
}

// TestEngine_ProviderPanic_FailsTheJobAndKeepsThePoolUsable covers the reach gap
// the job runner's recovery cannot close: the fan-out runs each provider on its
// own goroutine, so a panic there never reaches Pool.run. Before the guard the
// process died and the job stayed running forever, which is the outcome the
// runner's recovery exists to prevent.
func TestEngine_ProviderPanic_FailsTheJobAndKeepsThePoolUsable(t *testing.T) {
	repo := &mockRepo{}
	logger, sink := newRecordingLogger()
	eng := NewEngine(
		staticProviders(&panickingProvider{name: "expenses"}),
		nopFinance{}, repo, newMockSender(), 5, 5*time.Minute, logger,
	)

	eng.Submit("job-panic", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "job-panic", failed[0].JobID)
	assert.Contains(t, failed[0].ErrMsg, "Failed to collect expenses data",
		"a panicking provider must take the same terminal path as a failing one, naming the provider")
	assert.NotContains(t, failed[0].ErrMsg, "user@example.com",
		"the panic value never reaches the reason datarights shows the user")
	assert.Empty(t, repo.getCompletedJobs())

	record := findRecord(t, errorRecords(t, sink), "recovered panic in provider collection")
	assert.Equal(t, "panic: provider exploded holding user@example.com", record["panic"])
	assert.Equal(t, "job-panic", record["job_id"])
	assert.Equal(t, "user-1", record["user_id"])
	assert.Equal(t, "expenses", record["provider"])
	// The panicking frame, not debug.Stack's own first frame: a stack holding only
	// recovery machinery is useless and must fail here.
	assert.Contains(t, record["stack"], "panickingProvider")

	// A second job proves the pool survived the first panic and still runs work.
	eng.Submit("job-after", "user-1", "")
	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 2
	}, 2*time.Second, 10*time.Millisecond, "the pool must stay usable after a provider panic")
}

// TestEngine_OneProviderPanics_SiblingsStillReportTheFailure pins that the
// panic reaches Wait as the first error, so the job's reason names the provider
// that actually panicked rather than a sibling cancelled by errgroup.
func TestEngine_OneProviderPanics_SiblingsStillReportTheFailure(t *testing.T) {
	repo := &mockRepo{}
	logger, sink := newRecordingLogger()
	eng := NewEngine(
		staticProviders(
			&panickingProvider{name: "expenses"},
			&stubProvider{name: "profile", headers: []string{"username"}, rows: [][]string{{"alex"}}},
		),
		nopFinance{}, repo, newMockSender(), 5, 5*time.Minute, logger,
	)

	eng.Submit("job-mixed", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.Contains(t, repo.getFailedJobs()[0].ErrMsg, "Failed to collect expenses data")
	record := findRecord(t, errorRecords(t, sink), "recovered panic in provider collection")
	assert.Equal(t, "expenses", record["provider"])
}
