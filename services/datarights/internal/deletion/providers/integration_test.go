package providers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

// ---------------------------------------------------------------------------
// Integration mock repo: tracks state transitions
// ---------------------------------------------------------------------------

type integrationMockRepo struct {
	mu             sync.Mutex
	statusUpdates  []string
	completedJobs  []string
	failedJobs     []failedJobRecord
	nonTerminalRes []model.RecoverableDeletionJob
	nonTerminalErr error
}

type failedJobRecord struct {
	JobID  string
	ErrMsg string
}

func (m *integrationMockRepo) CreateJob(_ context.Context, _, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *integrationMockRepo) GetJob(_ context.Context, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *integrationMockRepo) GetInProgressJob(_ context.Context, _ string) (*model.DeletionJob, error) {
	return nil, nil
}

func (m *integrationMockRepo) UpdateStatus(_ context.Context, _ string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusUpdates = append(m.statusUpdates, status)
	return nil
}

func (m *integrationMockRepo) CompleteJob(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedJobs = append(m.completedJobs, jobID)
	return nil
}

func (m *integrationMockRepo) FailJob(_ context.Context, jobID, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedJobs = append(m.failedJobs, failedJobRecord{JobID: jobID, ErrMsg: errMsg})
	return nil
}

func (m *integrationMockRepo) GetNonTerminalJobs(_ context.Context) ([]model.RecoverableDeletionJob, error) {
	return m.nonTerminalRes, m.nonTerminalErr
}

// ---------------------------------------------------------------------------
// Order-tracking provider wrapper
// ---------------------------------------------------------------------------

type orderTrackingProvider struct {
	inner    deletion.DeletionProvider
	mu       *sync.Mutex
	callLog  *[]string
	calledAt *[]time.Time
}

func (p *orderTrackingProvider) Name() string {
	return p.inner.Name()
}

func (p *orderTrackingProvider) Delete(ctx context.Context, userID string) error {
	p.mu.Lock()
	*p.callLog = append(*p.callLog, p.inner.Name())
	*p.calledAt = append(*p.calledAt, time.Now())
	p.mu.Unlock()
	return p.inner.Delete(ctx, userID)
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

func TestIntegration_AllProviders_CalledInOrder_JobCompletes(t *testing.T) {
	financeClient := &mockFinanceClient{}
	expenseClient := &mockExpenseClient{}
	authClient := &mockAuthClient{}

	financeProv := NewFinanceDeletionProvider(financeClient)
	expenseProv := NewExpenseDeletionProvider(expenseClient)
	authProv := NewAuthDeletionProvider(authClient)

	// Wrap in order-tracking providers
	var mu sync.Mutex
	var callLog []string
	var calledAt []time.Time

	wrappedFinance := &orderTrackingProvider{inner: financeProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}
	wrappedExpense := &orderTrackingProvider{inner: expenseProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}
	wrappedAuth := &orderTrackingProvider{inner: authProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}

	// Register in correct order: finance → expense → auth
	registry := deletion.NewDeletionProviderRegistry()
	registry.Register(wrappedFinance)
	registry.Register(wrappedExpense)
	registry.Register(wrappedAuth)

	repo := &integrationMockRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	engine := deletion.NewDeletionEngine(registry, repo, 2, 30*time.Second, logger)

	// Submit a job
	engine.Submit("job-1", "user-42")

	// Wait for completion
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.completedJobs) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// Verify all providers were called
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, callLog, 3)
	assert.Equal(t, []string{"finance", "expense", "auth"}, callLog)

	// Verify correct user ID was passed to each client
	assert.Equal(t, "user-42", financeClient.deleteCalledWith)
	assert.Equal(t, "user-42", expenseClient.anonymizeCalledWith)
	assert.Equal(t, "user-42", authClient.deleteUserDataCalledWith)

	// Verify job was completed
	repo.mu.Lock()
	assert.Equal(t, []string{"job-1"}, repo.completedJobs)
	repo.mu.Unlock()

	// Verify status transitioned to running first
	repo.mu.Lock()
	require.Len(t, repo.statusUpdates, 1)
	assert.Equal(t, "running", repo.statusUpdates[0])
	repo.mu.Unlock()
}

func TestIntegration_ProviderFailure_JobFails_RemainingSkipped(t *testing.T) {
	financeClient := &mockFinanceClient{}
	expenseClient := &mockExpenseClient{
		anonymizeErr: fmt.Errorf("expense service down"),
	}
	authClient := &mockAuthClient{}

	financeProv := NewFinanceDeletionProvider(financeClient)
	expenseProv := NewExpenseDeletionProvider(expenseClient)
	authProv := NewAuthDeletionProvider(authClient)

	var mu sync.Mutex
	var callLog []string
	var calledAt []time.Time

	wrappedFinance := &orderTrackingProvider{inner: financeProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}
	wrappedExpense := &orderTrackingProvider{inner: expenseProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}
	wrappedAuth := &orderTrackingProvider{inner: authProv, mu: &mu, callLog: &callLog, calledAt: &calledAt}

	registry := deletion.NewDeletionProviderRegistry()
	registry.Register(wrappedFinance)
	registry.Register(wrappedExpense)
	registry.Register(wrappedAuth)

	repo := &integrationMockRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	engine := deletion.NewDeletionEngine(registry, repo, 2, 30*time.Second, logger)

	engine.Submit("job-2", "user-99")

	// Wait for failure (expense will retry 3 times with backoff)
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.failedJobs) == 1
	}, 10*time.Second, 50*time.Millisecond)

	// Verify finance was called but auth was NOT (fail-fast after expense)
	mu.Lock()
	assert.Contains(t, callLog, "finance")
	assert.Contains(t, callLog, "expense")
	assert.NotContains(t, callLog, "auth")
	mu.Unlock()

	// Verify auth was never called
	assert.Empty(t, authClient.deleteUserDataCalledWith)

	// Verify the failure message references the expense provider
	repo.mu.Lock()
	require.Len(t, repo.failedJobs, 1)
	assert.Contains(t, repo.failedJobs[0].ErrMsg, "expense")
	assert.Contains(t, repo.failedJobs[0].ErrMsg, "expense service down")
	repo.mu.Unlock()
}
