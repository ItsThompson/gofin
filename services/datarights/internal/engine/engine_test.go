package engine

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

	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

// mockRepo implements repository.JobRepository for engine tests.
type mockRepo struct {
	mu             sync.Mutex
	statusUpdates  []statusUpdate
	completedJobs  []completedJob
	failedJobs     []failedJob
	nonTerminalRes []model.RecoverableJob
	nonTerminalErr error
}

type statusUpdate struct {
	JobID  string
	Status string
}

type completedJob struct {
	JobID         string
	FileSizeBytes int64
}

type failedJob struct {
	JobID  string
	ErrMsg string
}

func (m *mockRepo) CreateJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}

func (m *mockRepo) GetJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}

func (m *mockRepo) ListJobsByUser(_ context.Context, _ string, _, _ int) ([]*model.ExportJob, int64, error) {
	return nil, 0, nil
}

func (m *mockRepo) GetInProgressJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}

func (m *mockRepo) GetLatestNonFailedJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}

func (m *mockRepo) UpdateStatus(_ context.Context, jobID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusUpdates = append(m.statusUpdates, statusUpdate{JobID: jobID, Status: status})
	return nil
}

func (m *mockRepo) CompleteJob(_ context.Context, jobID string, fileSizeBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedJobs = append(m.completedJobs, completedJob{JobID: jobID, FileSizeBytes: fileSizeBytes})
	return nil
}

func (m *mockRepo) FailJob(_ context.Context, jobID, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedJobs = append(m.failedJobs, failedJob{JobID: jobID, ErrMsg: errMsg})
	return nil
}

func (m *mockRepo) GetNonTerminalJobs(_ context.Context) ([]model.RecoverableJob, error) {
	return m.nonTerminalRes, m.nonTerminalErr
}

func (m *mockRepo) getStatusUpdates() []statusUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]statusUpdate, len(m.statusUpdates))
	copy(result, m.statusUpdates)
	return result
}

func (m *mockRepo) getCompletedJobs() []completedJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]completedJob, len(m.completedJobs))
	copy(result, m.completedJobs)
	return result
}

func (m *mockRepo) getFailedJobs() []failedJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]failedJob, len(m.failedJobs))
	copy(result, m.failedJobs)
	return result
}

// stubProvider implements DataProvider for testing.
type stubProvider struct {
	name    string
	headers []string
	rows    [][]string
	err     error
	delay   time.Duration
}

func (p *stubProvider) Name() string      { return p.name }
func (p *stubProvider) Headers() []string  { return p.headers }
func (p *stubProvider) Collect(ctx context.Context, _ string) ([][]string, error) {
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockSender implements email.Sender for engine tests.
type mockSender struct {
	mu       sync.Mutex
	calls    []mockSendCall
	err      error
}

type mockSendCall struct {
	ToEmail  string
	ZipSize  int
}

var _ email.Sender = (*mockSender)(nil)

func (m *mockSender) SendExportEmail(_ context.Context, toEmail string, zipBytes []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockSendCall{ToEmail: toEmail, ZipSize: len(zipBytes)})
	return m.err
}

func (m *mockSender) getCalls() []mockSendCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockSendCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func newMockSender() *mockSender {
	return &mockSender{}
}

func TestEngine_HappyPath_CompletesJob(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"username", "email"},
		rows:    [][]string{{"alex", "alex@example.com"}},
	})

	eng := NewEngine(registry, repo, newMockSender(), 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-1", "user-1", "alex@example.com")

	// Wait for async completion
	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Verify status transition: pending → running
	updates := repo.getStatusUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, "job-1", updates[0].JobID)
	assert.Equal(t, "running", updates[0].Status)

	// Verify completion
	completed := repo.getCompletedJobs()
	assert.Equal(t, "job-1", completed[0].JobID)
	assert.Greater(t, completed[0].FileSizeBytes, int64(0))
}

func TestEngine_ProviderError_FailsJob(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name: "profile",
		err:  fmt.Errorf("gRPC unavailable"),
	})

	eng := NewEngine(registry, repo, newMockSender(), 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-2", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "job-2", failed[0].JobID)
	assert.Contains(t, failed[0].ErrMsg, "Failed to collect profile data")
	// Verify no PII or stack traces in error message
	assert.NotContains(t, failed[0].ErrMsg, "gRPC unavailable")
}

func TestEngine_Timeout_FailsJob(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "slow-provider",
		headers: []string{"col"},
		rows:    [][]string{{"val"}},
		delay:   500 * time.Millisecond,
	})

	// Very short timeout to trigger
	eng := NewEngine(registry, repo, newMockSender(), 5, 50*time.Millisecond, newTestLogger())
	eng.Submit("job-3", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Equal(t, "job-3", failed[0].JobID)
	assert.Equal(t, "Export timed out", failed[0].ErrMsg)
}

func TestEngine_ConcurrencyBounded(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()

	var running atomic.Int32
	var maxSeen atomic.Int32

	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"col"},
		rows:    [][]string{{"val"}},
		delay:   100 * time.Millisecond,
	})

	maxConcurrent := 3
	eng := NewEngine(registry, repo, newMockSender(), maxConcurrent, 5*time.Minute, newTestLogger())

	// Override the execute tracking with a custom provider that monitors concurrency
	trackingRegistry := NewProviderRegistry()
	trackingRegistry.Register(&concurrencyTrackingProvider{
		running: &running,
		maxSeen: &maxSeen,
		delay:   100 * time.Millisecond,
	})
	eng.registry = trackingRegistry

	// Submit more jobs than max concurrent
	totalJobs := 10
	for i := range totalJobs {
		eng.Submit(fmt.Sprintf("job-%d", i), "user-1", "")
	}

	// Wait for all jobs to complete
	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == totalJobs
	}, 5*time.Second, 50*time.Millisecond)

	// Verify concurrency was bounded
	assert.LessOrEqual(t, int(maxSeen.Load()), maxConcurrent,
		"concurrent goroutines exceeded max: saw %d, limit %d", maxSeen.Load(), maxConcurrent)
}

// concurrencyTrackingProvider tracks concurrent execution count.
type concurrencyTrackingProvider struct {
	running *atomic.Int32
	maxSeen *atomic.Int32
	delay   time.Duration
}

func (p *concurrencyTrackingProvider) Name() string      { return "profile" }
func (p *concurrencyTrackingProvider) Headers() []string  { return []string{"col"} }
func (p *concurrencyTrackingProvider) Collect(ctx context.Context, _ string) ([][]string, error) {
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
		return nil, ctx.Err()
	}

	return [][]string{{"val"}}, nil
}

func TestEngine_MultipleProviders_AllCollected(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"username"},
		rows:    [][]string{{"alex"}},
	})
	registry.Register(&stubProvider{
		name:    "expenses",
		headers: []string{"id", "amount"},
		rows: [][]string{
			{"1", "45.99"},
			{"2", "30.00"},
		},
	})

	eng := NewEngine(registry, repo, newMockSender(), 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-multi", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// The file size should reflect both CSV files in the ZIP
	completed := repo.getCompletedJobs()
	assert.Greater(t, completed[0].FileSizeBytes, int64(0))
}

func TestEngine_SecondProviderError_FailsJob(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"username"},
		rows:    [][]string{{"alex"}},
	})
	registry.Register(&stubProvider{
		name: "expenses",
		err:  fmt.Errorf("upstream timeout"),
	})

	eng := NewEngine(registry, repo, newMockSender(), 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-fail", "user-1", "")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Contains(t, failed[0].ErrMsg, "Failed to collect expenses data")
}

func TestEngine_EmailSenderCalled_OnSuccess(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"username", "email"},
		rows:    [][]string{{"alex", "alex@example.com"}},
	})

	sender := newMockSender()
	eng := NewEngine(registry, repo, sender, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-email", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getCompletedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Verify email was sent
	calls := sender.getCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "alex@example.com", calls[0].ToEmail)
	assert.Greater(t, calls[0].ZipSize, 0)
}

func TestEngine_EmailFailure_FailsJob(t *testing.T) {
	repo := &mockRepo{}
	registry := NewProviderRegistry()
	registry.Register(&stubProvider{
		name:    "profile",
		headers: []string{"username"},
		rows:    [][]string{{"alex"}},
	})

	sender := &mockSender{err: fmt.Errorf("Resend API error (status 429): rate limited")}
	eng := NewEngine(registry, repo, sender, 5, 5*time.Minute, newTestLogger())
	eng.Submit("job-email-fail", "user-1", "alex@example.com")

	require.Eventually(t, func() bool {
		return len(repo.getFailedJobs()) == 1
	}, 2*time.Second, 10*time.Millisecond)

	failed := repo.getFailedJobs()
	assert.Contains(t, failed[0].ErrMsg, "Email delivery failed")
	// Job should not be completed when email fails
	assert.Len(t, repo.getCompletedJobs(), 0)
}
