package jobrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
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

// ---------------------------------------------------------------------------
// Panic containment
// ---------------------------------------------------------------------------

func TestPool_ExecutePanic_FailsTheJobWithAPIIFreeReason(t *testing.T) {
	store := &fakeStore{}
	// A 50ms job timeout, mirroring the timeout test: the strategy sleeps past it
	// before panicking, so the job context is provably expired by the time the
	// recovery runs. Without a fresh background context FailJob would see a dead
	// one, which makes the ctxLive assertion below falsifiable.
	execute := func(ctx context.Context, _, _ string) error {
		<-ctx.Done()
		panic("strategy exploded holding user@example.com")
	}

	pool := New(5, 50*time.Millisecond, store, execute, testLogger())
	pool.Submit("job-panic", "user-1")

	require.Eventually(t, func() bool {
		return len(store.failedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := store.failedSnapshot()
	assert.Equal(t, "job-panic", failed[0].jobID)
	assert.Equal(t, "Job failed unexpectedly", failed[0].reason)
	assert.NotContains(t, failed[0].reason, "user@example.com",
		"the panic value never reaches the reason datarights shows the user")
	assert.True(t, failed[0].ctxLive,
		"the job context is expired, so FailJob must use a fresh background context")
	assert.Equal(t, []statusCall{{jobID: "job-panic", status: "running"}}, store.statusSnapshot())
	assert.Empty(t, store.completedSnapshot(), "a panicking job must not be completed")

	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 0
	}, 2*time.Second, 10*time.Millisecond, "the slot must be released after the recovery runs")
}

func TestPool_ExecutePanic_WritesOneErrorRecordWithPanicAndStack(t *testing.T) {
	store := &fakeStore{}
	logger, logs := serverkittest.NewLogger()

	pool := New(5, time.Minute, store, panickingExecute, logger)
	pool.Submit("job-panic", "user-7")

	require.Eventually(t, func() bool {
		return len(store.failedSnapshot()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	records, err := logs.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1, "a recovered panic must produce exactly one error-level record")
	assert.Equal(t, "ERROR", records[0]["level"])
	assert.Equal(t, "recovered panic in job execution", records[0]["msg"])
	assert.Equal(t, "panic: strategy exploded", records[0]["panic"])
	assert.Equal(t, "job-panic", records[0]["job_id"])
	assert.Equal(t, "user-7", records[0]["user_id"])
	// The panicking frame, not debug.Stack's own first frame: a stack holding only
	// recovery machinery is useless and must fail here.
	assert.Contains(t, records[0]["stack"], "panickingExecute")
}

// panickingExecute is a named Execute strategy so the recorded stack carries a
// frame to assert on.
func panickingExecute(context.Context, string, string) error { panic("strategy exploded") }

// TestPool_ExecutePanic_AtCapacity_LeavesThePoolUsable guards the interaction
// between the new recovery defer and the two existing ones. With a single slot,
// a panicking job that failed to release it would starve every queued job, so
// the three survivors completing is what proves the LIFO ordering holds.
func TestPool_ExecutePanic_AtCapacity_LeavesThePoolUsable(t *testing.T) {
	store := &fakeStore{}

	var started atomic.Int32
	execute := func(context.Context, string, string) error {
		if started.Add(1) == 1 {
			panic("first job exploded")
		}
		return nil
	}

	const total = 4
	pool := New(1, time.Minute, store, execute, testLogger())
	for i := 0; i < total; i++ {
		pool.Submit(fmt.Sprintf("job-%d", i), "user-1")
	}

	require.Eventually(t, func() bool {
		return len(store.completedSnapshot()) == total-1 && len(store.failedSnapshot()) == 1
	}, 5*time.Second, 20*time.Millisecond, "every queued job must still get the slot")

	require.Eventually(t, func() bool {
		return pool.ActiveJobs() == 0 && pool.QueuedJobs() == 0
	}, 2*time.Second, 10*time.Millisecond, "the pool must drain after a panic")
}
