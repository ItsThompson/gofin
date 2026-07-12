package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// capturingSender records the ZIP bytes handed to the last successful send so
// fan-out tests can inspect the assembled archive (e.g. file order).
type capturingSender struct {
	mu  sync.Mutex
	zip []byte
}

var _ email.Sender = (*capturingSender)(nil)

func (s *capturingSender) SendExportEmail(_ context.Context, _ string, zipBytes []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zip = append([]byte(nil), zipBytes...)
	return nil
}

func (s *capturingSender) capturedNames(t *testing.T) []string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	require.NotEmpty(t, s.zip, "no ZIP was captured")

	zr, err := zip.NewReader(bytes.NewReader(s.zip), int64(len(s.zip)))
	require.NoError(t, err)
	names := make([]string, len(zr.File))
	for i, f := range zr.File {
		names[i] = f.Name
	}
	return names
}

// TestEngine_FanOut_ZIPOrderDeterministic proves the index-addressed writes keep
// ZIP order equal to registration order even when providers finish in the
// reverse order. Provider 0 is the slowest and provider 4 the fastest, so
// completion order is the reverse of registration order; the archive must still
// be p0..p4.
func TestEngine_FanOut_ZIPOrderDeterministic(t *testing.T) {
	const n = 5
	provs := make([]DataProvider, n)
	for i := range provs {
		provs[i] = &stubProvider{
			name:    fmt.Sprintf("p%d", i),
			headers: []string{"col"},
			rows:    [][]string{{fmt.Sprintf("val-%d", i)}},
			delay:   time.Duration(n-i) * 8 * time.Millisecond, // p0 slowest, p4 fastest
		}
	}

	repo := &mockRepo{}
	sender := &capturingSender{}
	eng := NewEngine(staticProviders(provs...), nopFinance{}, repo, sender, 5, time.Minute, newTestLogger())
	eng.Submit("job-order", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond, "job did not complete; failures=%v", repo.getFailedJobs())

	assert.Equal(t,
		[]string{"p0.csv", "p1.csv", "p2.csv", "p3.csv", "p4.csv"},
		sender.capturedNames(t),
		"ZIP order must match registration order regardless of completion order")
}

// TestEngine_FanOut_RunsProvidersConcurrently is the deterministic, machine-
// independent form of "max, not sum": it asserts every provider is in flight at
// the same time. A serial loop would peak at one concurrent provider.
func TestEngine_FanOut_RunsProvidersConcurrently(t *testing.T) {
	const n = 5
	var running, maxSeen atomic.Int32
	provs := make([]DataProvider, n)
	for i := range provs {
		provs[i] = &concurrencyTrackingProvider{
			running: &running,
			maxSeen: &maxSeen,
			delay:   40 * time.Millisecond,
		}
	}

	repo := &mockRepo{}
	eng := NewEngine(staticProviders(provs...), nopFinance{}, repo, newMockSender(), 5, time.Minute, newTestLogger())
	eng.Submit("job-concurrent", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond, "job did not complete; failures=%v", repo.getFailedJobs())

	assert.Equal(t, int32(n), maxSeen.Load(),
		"all %d providers must run concurrently under the fan-out; saw peak %d", n, maxSeen.Load())
}

// TestEngine_FanOut_NamedProviderErrorSurvivesFirstError proves the provider
// name is baked into the error inside the goroutine, so the human-readable
// "Failed to collect X data" message survives errgroup's first-error capture,
// and the underlying cause is not leaked. The failing provider is not the first
// in registration order and returns quickly so its error is the one captured.
func TestEngine_FanOut_NamedProviderErrorSurvivesFirstError(t *testing.T) {
	repo := &mockRepo{}
	eng := NewEngine(staticProviders(
		&stubProvider{name: "profile", headers: []string{"c"}, rows: [][]string{{"v"}}, delay: 50 * time.Millisecond},
		&stubProvider{name: "expenses", err: fmt.Errorf("gRPC unavailable: connection refused")},
		&stubProvider{name: "tags", headers: []string{"c"}, rows: [][]string{{"v"}}, delay: 50 * time.Millisecond},
	), nopFinance{}, repo, newMockSender(), 5, time.Minute, newTestLogger())
	eng.Submit("job-named-err", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "Failed to collect expenses data", failed[0].ErrMsg)
	assert.NotContains(t, failed[0].ErrMsg, "connection refused", "must not leak the underlying cause")
}

// TestEngine_FanOut_TimeoutMapsToExportTimedOut proves the post-Wait context
// recheck: when the job context expires, the wrapped context.DeadlineExceeded
// error a provider returns is classified as a timeout ("Export timed out"),
// not a plain collect failure.
func TestEngine_FanOut_TimeoutMapsToExportTimedOut(t *testing.T) {
	repo := &mockRepo{}
	eng := NewEngine(staticProviders(
		&stubProvider{name: "profile", headers: []string{"c"}, rows: [][]string{{"v"}}},
		&stubProvider{name: "expenses", headers: []string{"c"}, rows: [][]string{{"v"}}, delay: 500 * time.Millisecond},
	), nopFinance{}, repo, newMockSender(), 5, 50*time.Millisecond, newTestLogger())
	eng.Submit("job-timeout", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "Export timed out", failed[0].ErrMsg)
}

// ctxCapturingRepo records whether the context passed to FailJob is still live.
// It embeds the JobRepository interface (nil) and overrides only the methods
// execute exercises, mirroring the finance-spy convention in this package.
type ctxCapturingRepo struct {
	repository.JobRepository

	mu          sync.Mutex
	failMsg     string
	failCtxLive bool
	failed      bool
}

func (r *ctxCapturingRepo) UpdateStatus(_ context.Context, _, _ string) error { return nil }

func (r *ctxCapturingRepo) CompleteJob(_ context.Context, _ string, _ int64) error { return nil }

func (r *ctxCapturingRepo) FailJob(ctx context.Context, _ string, msg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = true
	r.failMsg = msg
	r.failCtxLive = ctx.Err() == nil
	return nil
}

func (r *ctxCapturingRepo) snapshot() (bool, bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failed, r.failCtxLive, r.failMsg
}

// TestEngine_FanOut_FailurePersistsViaBackgroundContext proves failJob writes
// the failure with a fresh context.Background(), so the failure DB write still
// succeeds after the job context has expired.
func TestEngine_FanOut_FailurePersistsViaBackgroundContext(t *testing.T) {
	repo := &ctxCapturingRepo{}
	eng := NewEngine(staticProviders(
		&stubProvider{name: "expenses", headers: []string{"c"}, rows: [][]string{{"v"}}, delay: 500 * time.Millisecond},
	), nopFinance{}, repo, newMockSender(), 5, 50*time.Millisecond, newTestLogger())
	eng.Submit("job-persist", "user-1", "")

	require.Eventually(t, func() bool {
		failed, _, _ := repo.snapshot()
		return failed
	}, 2*time.Second, 10*time.Millisecond)

	failed, ctxLive, msg := repo.snapshot()
	require.True(t, failed)
	assert.True(t, ctxLive, "failJob must use a fresh context that survives the expired job context")
	assert.Equal(t, "Export timed out", msg)
}
