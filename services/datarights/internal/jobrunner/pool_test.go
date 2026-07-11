package jobrunner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fake StatusStore
// ---------------------------------------------------------------------------

// fakeStore is a thread-safe StatusStore that records every transition. It is a
// real collaborator for the pool: the tests exercise the actual pool goroutine,
// semaphore, and context handling against it (no mocking of the pool internals).
type fakeStore struct {
	mu            sync.Mutex
	statusUpdates []statusCall
	completed     []string
	failed        []failCall

	updateErr   error
	completeErr error
}

type statusCall struct {
	jobID  string
	status string
}

type failCall struct {
	jobID   string
	reason  string
	ctxLive bool // whether the context passed to FailJob was still live
}

func (s *fakeStore) UpdateStatus(_ context.Context, jobID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusUpdates = append(s.statusUpdates, statusCall{jobID: jobID, status: status})
	return s.updateErr
}

func (s *fakeStore) CompleteJob(_ context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completeErr != nil {
		return s.completeErr
	}
	s.completed = append(s.completed, jobID)
	return nil
}

func (s *fakeStore) FailJob(ctx context.Context, jobID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, failCall{jobID: jobID, reason: reason, ctxLive: ctx.Err() == nil})
	return nil
}

func (s *fakeStore) statusSnapshot() []statusCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]statusCall(nil), s.statusUpdates...)
}

func (s *fakeStore) completedSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.completed...)
}

func (s *fakeStore) failedSnapshot() []failCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]failCall(nil), s.failed...)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Lifecycle transitions
// ---------------------------------------------------------------------------

func TestPool_Success_TransitionsToRunningThenCompletes(t *testing.T) {
	store := &fakeStore{}
	var executed atomic.Bool
	execute := func(_ context.Context, _, _ string) error {
		executed.Store(true)
		return nil
	}

	pool := New(5, time.Minute, store, execute, testLogger())
	pool.Submit("job-1", "user-1")

	require.Eventually(t, func() bool {
		return len(store.completedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	assert.True(t, executed.Load(), "execute strategy must run")
	assert.Equal(t, []statusCall{{jobID: "job-1", status: "running"}}, store.statusSnapshot())
	assert.Equal(t, []string{"job-1"}, store.completedSnapshot())
	assert.Empty(t, store.failedSnapshot())
}

func TestPool_ExecuteError_FailsJobWithBackgroundContext(t *testing.T) {
	store := &fakeStore{}
	execute := func(_ context.Context, _, _ string) error {
		return errors.New("strategy blew up")
	}

	pool := New(5, time.Minute, store, execute, testLogger())
	pool.Submit("job-err", "user-1")

	require.Eventually(t, func() bool {
		return len(store.failedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := store.failedSnapshot()
	assert.Equal(t, "job-err", failed[0].jobID)
	assert.Equal(t, "strategy blew up", failed[0].reason)
	assert.True(t, failed[0].ctxLive, "FailJob must run under a fresh background context")
	assert.Empty(t, store.completedSnapshot(), "a failed job must not be completed")
}

func TestPool_Timeout_FailsJobViaBackgroundContext(t *testing.T) {
	store := &fakeStore{}
	// The strategy honors the job context; with a tiny timeout it returns the
	// expired-context error, exactly like a real per-provider deadline.
	execute := func(ctx context.Context, _, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}

	pool := New(5, 50*time.Millisecond, store, execute, testLogger())
	pool.Submit("job-timeout", "user-1")

	require.Eventually(t, func() bool {
		return len(store.failedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := store.failedSnapshot()
	assert.Equal(t, "job-timeout", failed[0].jobID)
	assert.Equal(t, context.DeadlineExceeded.Error(), failed[0].reason)
	assert.True(t, failed[0].ctxLive,
		"the job context is expired, so FailJob must use a fresh background context")
	assert.Empty(t, store.completedSnapshot())
}

func TestPool_UpdateStatusError_SkipsExecuteAndTerminalWrites(t *testing.T) {
	store := &fakeStore{updateErr: errors.New("db down")}
	var executed atomic.Bool
	execute := func(_ context.Context, _, _ string) error {
		executed.Store(true)
		return nil
	}

	pool := New(5, time.Minute, store, execute, testLogger())
	pool.Submit("job-nostart", "user-1")

	require.Eventually(t, func() bool {
		return len(store.statusSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)
	// Give the worker goroutine room to (incorrectly) proceed if the guard fails.
	time.Sleep(50 * time.Millisecond)

	assert.False(t, executed.Load(), "execute must not run when the running transition fails")
	assert.Empty(t, store.completedSnapshot())
	assert.Empty(t, store.failedSnapshot())
}

// ---------------------------------------------------------------------------
// Concurrency and telemetry
// ---------------------------------------------------------------------------

func TestPool_BoundsConcurrencyToMaxConcurrent(t *testing.T) {
	const maxConcurrent = 3
	store := &fakeStore{}

	var running, maxSeen atomic.Int32
	execute := func(_ context.Context, _, _ string) error {
		cur := running.Add(1)
		defer running.Add(-1)
		for {
			prev := maxSeen.Load()
			if cur <= prev || maxSeen.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		return nil
	}

	pool := New(maxConcurrent, time.Minute, store, execute, testLogger())
	assert.Equal(t, maxConcurrent, pool.MaxConcurrent())

	const total = 12
	for i := 0; i < total; i++ {
		pool.Submit("job", "user-1")
	}

	require.Eventually(t, func() bool {
		return len(store.completedSnapshot()) == total
	}, 5*time.Second, 20*time.Millisecond)

	assert.LessOrEqual(t, int(maxSeen.Load()), maxConcurrent,
		"in-flight jobs exceeded MaxConcurrent: saw %d, limit %d", maxSeen.Load(), maxConcurrent)
}

func TestPool_ActiveJobs_ReflectsInFlightJobs(t *testing.T) {
	store := &fakeStore{}
	release := make(chan struct{})
	execute := func(_ context.Context, _, _ string) error {
		<-release // hold the slot until the test releases it
		return nil
	}

	pool := New(5, time.Minute, store, execute, testLogger())
	for i := 0; i < 3; i++ {
		pool.Submit("job", "user-1")
	}

	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 3
	}, 2*time.Second, 10*time.Millisecond, "expected 3 in-flight jobs")

	close(release)

	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 0 && len(store.completedSnapshot()) == 3
	}, 2*time.Second, 10*time.Millisecond, "in-flight count should drain to 0 after release")
}

func TestPool_QueuedJobs_ReflectsJobsWaitingForASlot(t *testing.T) {
	store := &fakeStore{}
	release := make(chan struct{})
	execute := func(_ context.Context, _, _ string) error {
		<-release
		return nil
	}

	pool := New(2, time.Minute, store, execute, testLogger())
	for i := 0; i < 5; i++ {
		pool.Submit("job", "user-1")
	}

	// 2 jobs hold slots; the remaining 3 are queued waiting for one.
	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 2 && pool.QueuedJobs() == 3
	}, 2*time.Second, 10*time.Millisecond, "expected 2 active and 3 queued")

	close(release)

	require.Eventually(t, func() bool {
		return pool.QueuedJobs() == 0 && pool.ActiveJobs() == 0 && len(store.completedSnapshot()) == 5
	}, 2*time.Second, 10*time.Millisecond)
}
