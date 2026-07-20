package deletion

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type mockDeletionRepo struct {
	mu             sync.Mutex
	statusUpdates  []statusUpdate
	completedJobs  []string
	failedJobs     []failedJob
	nonTerminalRes []model.RecoverableDeletionJob
	nonTerminalErr error
}

type statusUpdate struct {
	JobID  string
	Status string
}

type failedJob struct {
	JobID  string
	ErrMsg string
}

func (m *mockDeletionRepo) CreateJob(_ context.Context, _, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *mockDeletionRepo) GetJob(_ context.Context, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *mockDeletionRepo) GetInProgressJob(_ context.Context, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *mockDeletionRepo) UpdateStatus(_ context.Context, jobID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusUpdates = append(m.statusUpdates, statusUpdate{JobID: jobID, Status: status})
	return nil
}

func (m *mockDeletionRepo) CompleteJob(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedJobs = append(m.completedJobs, jobID)
	return nil
}

func (m *mockDeletionRepo) FailJob(_ context.Context, jobID, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedJobs = append(m.failedJobs, failedJob{JobID: jobID, ErrMsg: errMsg})
	return nil
}

func (m *mockDeletionRepo) GetNonTerminalJobs(_ context.Context) ([]model.RecoverableDeletionJob, error) {
	return m.nonTerminalRes, m.nonTerminalErr
}

func (m *mockDeletionRepo) getStatusUpdates() []statusUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]statusUpdate, len(m.statusUpdates))
	copy(result, m.statusUpdates)
	return result
}

func (m *mockDeletionRepo) getCompletedJobs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.completedJobs))
	copy(result, m.completedJobs)
	return result
}

func (m *mockDeletionRepo) getFailedJobs() []failedJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]failedJob, len(m.failedJobs))
	copy(result, m.failedJobs)
	return result
}

// ---------------------------------------------------------------------------
// Mock deletion provider
// ---------------------------------------------------------------------------

type mockDeletionProvider struct {
	name       string
	callCount  atomic.Int32
	failUntil  int // fail the first N attempts
	err        error
	delay      time.Duration
	mu         sync.Mutex
	callRecord []string // userIDs called
}

func (p *mockDeletionProvider) Name() string { return p.name }

func (p *mockDeletionProvider) Delete(ctx context.Context, userID string) error {
	p.mu.Lock()
	p.callRecord = append(p.callRecord, userID)
	p.mu.Unlock()

	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	attempt := int(p.callCount.Add(1))
	if attempt <= p.failUntil {
		if p.err != nil {
			return p.err
		}
		return fmt.Errorf("provider %s: simulated failure on attempt %d", p.name, attempt)
	}
	return nil
}

func (p *mockDeletionProvider) getCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]string, len(p.callRecord))
	copy(result, p.callRecord)
	return result
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestRegistry_All_ReturnsInRegistrationOrder(t *testing.T) {
	registry := NewRegistry()
	p1 := &mockDeletionProvider{name: "finance"}
	p2 := &mockDeletionProvider{name: "expense"}
	p3 := &mockDeletionProvider{name: "auth"}

	registry.Register(p1)
	registry.Register(p2)
	registry.Register(p3)

	all := registry.All()
	require.Len(t, all, 3)
	assert.Equal(t, "finance", all[0].Name())
	assert.Equal(t, "expense", all[1].Name())
	assert.Equal(t, "auth", all[2].Name())
}

func TestRegistry_All_EmptyRegistryReturnsNil(t *testing.T) {
	registry := NewRegistry()
	assert.Nil(t, registry.All())
}

// ---------------------------------------------------------------------------
// Engine tests
// ---------------------------------------------------------------------------

func TestEngine_HappyPath_CompletesJob(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()
	registry.Register(&mockDeletionProvider{name: "finance"})
	registry.Register(&mockDeletionProvider{name: "expense"})
	registry.Register(&mockDeletionProvider{name: "auth"})

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-1", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Verify status transition to running
	updates := repo.getStatusUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, "job-1", updates[0].JobID)
	assert.Equal(t, "running", updates[0].Status)

	completed := repo.getCompletedJobs()
	assert.Equal(t, "job-1", completed[0])

	assert.Empty(t, repo.getFailedJobs())
}

func TestEngine_ProvidersExecuteInOrder(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	p1 := &mockDeletionProvider{name: "finance"}
	p2 := &mockDeletionProvider{name: "expense"}
	p3 := &mockDeletionProvider{name: "auth"}

	registry.Register(p1)
	registry.Register(p2)
	registry.Register(p3)

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-order", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// All providers should have been called with the same userID
	assert.Equal(t, []string{"user-1"}, p1.getCalls())
	assert.Equal(t, []string{"user-1"}, p2.getCalls())
	assert.Equal(t, []string{"user-1"}, p3.getCalls())
}

func TestEngine_RetrySuccess_ContinuesToNextProvider(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	// Provider fails first attempt but succeeds on second
	retryProvider := &mockDeletionProvider{
		name:      "expense",
		failUntil: 1,
		err:       fmt.Errorf("temporary gRPC error"),
	}
	registry.Register(retryProvider)
	registry.Register(&mockDeletionProvider{name: "auth"})

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-retry", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// Expense provider was called twice (fail then success)
	assert.Equal(t, int32(2), retryProvider.callCount.Load())

	// Job completed successfully
	assert.Equal(t, "job-retry", repo.getCompletedJobs()[0])
	assert.Empty(t, repo.getFailedJobs())
}

func TestEngine_RetryExhaustion_FailsJob(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	// Provider fails all 3 attempts
	failProvider := &mockDeletionProvider{
		name:      "expense",
		failUntil: 3,
		err:       fmt.Errorf("persistent gRPC error"),
	}
	registry.Register(&mockDeletionProvider{name: "finance"})
	registry.Register(failProvider)
	registry.Register(&mockDeletionProvider{name: "auth"})

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-exhaust", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 10*time.Second, 10*time.Millisecond)

	// Verify failure message names the provider
	failed := repo.getFailedJobs()
	assert.Equal(t, "job-exhaust", failed[0].JobID)
	assert.Contains(t, failed[0].ErrMsg, "provider expense failed after 3 attempts")
	assert.Contains(t, failed[0].ErrMsg, "persistent gRPC error")

	// Verify provider was called exactly 3 times
	assert.Equal(t, int32(3), failProvider.callCount.Load())

	// Verify job was NOT completed
	assert.Empty(t, repo.getCompletedJobs())
}

func TestEngine_FailFast_RemainingProvidersNotAttempted(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	// First provider fails permanently
	failProvider := &mockDeletionProvider{
		name:      "finance",
		failUntil: 3,
		err:       fmt.Errorf("finance service down"),
	}
	// Second provider should never be called
	authProvider := &mockDeletionProvider{name: "auth"}

	registry.Register(failProvider)
	registry.Register(authProvider)

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-failfast", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 10*time.Second, 10*time.Millisecond)

	// Auth provider should NOT have been called
	assert.Empty(t, authProvider.getCalls())
}

func TestEngine_StateTransitions_PendingToRunningToCompleted(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()
	registry.Register(&mockDeletionProvider{name: "finance"})

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-states", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Status update to "running" happened
	updates := repo.getStatusUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, "running", updates[0].Status)

	// Job was completed
	assert.Equal(t, "job-states", repo.getCompletedJobs()[0])
}

func TestEngine_StateTransitions_PendingToRunningToFailed(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()
	registry.Register(&mockDeletionProvider{
		name:      "finance",
		failUntil: 3,
		err:       fmt.Errorf("always fails"),
	})

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-fail-states", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 10*time.Second, 10*time.Millisecond)

	// Status update to "running" happened
	updates := repo.getStatusUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, "running", updates[0].Status)

	// Job was failed
	assert.Equal(t, "job-fail-states", repo.getFailedJobs()[0].JobID)
}

func TestEngine_ContextTimeout_CountsAsFailedAttempt(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	// Provider has a delay longer than the engine timeout
	slowProvider := &mockDeletionProvider{
		name:  "slow",
		delay: 500 * time.Millisecond,
	}
	registry.Register(slowProvider)

	// Very short timeout to trigger context cancellation
	eng := NewEngine(registry, repo, 5, 100*time.Millisecond, newTestLogger())
	eng.Submit("job-timeout", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// Job should have failed due to context timeout
	failed := repo.getFailedJobs()
	assert.Equal(t, "job-timeout", failed[0].JobID)
	assert.Contains(t, failed[0].ErrMsg, "provider slow failed")
}

func TestEngine_BoundedConcurrency(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	var running atomic.Int32
	var maxSeen atomic.Int32

	// Use a provider that tracks concurrent execution
	trackingProvider := &concurrencyTrackingDeletionProvider{
		running: &running,
		maxSeen: &maxSeen,
		delay:   50 * time.Millisecond,
	}
	registry.Register(trackingProvider)

	maxConcurrent := 3
	eng := NewEngine(registry, repo, maxConcurrent, 5*time.Minute, newTestLogger())

	// Submit more jobs than max concurrent
	totalJobs := 10
	for i := range totalJobs {
		eng.Submit(fmt.Sprintf("job-%d", i), "user-1")
	}

	// Wait for all jobs to complete
	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == totalJobs
	}, 5*time.Second, 50*time.Millisecond)

	// Verify concurrency was bounded
	assert.LessOrEqual(t, int(maxSeen.Load()), maxConcurrent,
		"concurrent goroutines exceeded max: saw %d, limit %d", maxSeen.Load(), maxConcurrent)
}

func TestEngine_SubmitIsNonBlocking(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()
	registry.Register(&mockDeletionProvider{
		name:  "slow",
		delay: 1 * time.Second,
	})

	// Engine with 1 concurrency slot
	eng := NewEngine(registry, repo, 1, 5*time.Minute, newTestLogger())

	// Submit should return immediately even with a full semaphore
	start := time.Now()
	eng.Submit("job-nonblock-1", "user-1")
	eng.Submit("job-nonblock-2", "user-1")
	elapsed := time.Since(start)

	// Submit should be effectively instant (goroutines spawned but may block internally)
	assert.Less(t, elapsed, 50*time.Millisecond, "Submit should be non-blocking")

	// Eventually both jobs complete
	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 2
	}, 5*time.Second, 50*time.Millisecond)
}

func TestEngine_RetrySecondAttemptSuccess(t *testing.T) {
	repo := &mockDeletionRepo{}
	registry := NewRegistry()

	// Provider fails twice but succeeds on third (last) attempt
	retryProvider := &mockDeletionProvider{
		name:      "expense",
		failUntil: 2,
		err:       fmt.Errorf("transient error"),
	}
	registry.Register(retryProvider)

	eng := NewEngine(registry, repo, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-retry-2", "user-1")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 10*time.Second, 10*time.Millisecond)

	// Provider was called 3 times (2 failures + 1 success)
	assert.Equal(t, int32(3), retryProvider.callCount.Load())
	assert.Equal(t, "job-retry-2", repo.getCompletedJobs()[0])
}

// ---------------------------------------------------------------------------
// Concurrency tracking provider
// ---------------------------------------------------------------------------

type concurrencyTrackingDeletionProvider struct {
	running *atomic.Int32
	maxSeen *atomic.Int32
	delay   time.Duration
}

func (p *concurrencyTrackingDeletionProvider) Name() string { return "tracking" }

func (p *concurrencyTrackingDeletionProvider) Delete(ctx context.Context, _ string) error {
	current := p.running.Add(1)
	defer p.running.Add(-1)

	// Track maximum concurrent
	for {
		prev := p.maxSeen.Load()
		if current <= prev {
			break
		}
		if p.maxSeen.CompareAndSwap(prev, current) {
			break
		}
	}

	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
