package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	financeconfig "github.com/ItsThompson/gofin/services/finance/config"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

func TestExpenseRevisionOwnerUsesProcessWideMonotonicTokens(t *testing.T) {
	owner := newExpenseRevisionOwner()

	first := owner.observe("user-1")
	secondUser := owner.observe("user-2")
	afterMutation := owner.advance("user-1")
	owner.retire("user-1")
	recreated := owner.observe("user-1")

	require.NotEmpty(t, first.Epoch)
	assert.Equal(t, first.Epoch, secondUser.Epoch)
	assert.Equal(t, first.Epoch, afterMutation.Epoch)
	assert.Equal(t, first.Epoch, recreated.Epoch)
	assert.Equal(t, uint64(1), first.Revision)
	assert.Equal(t, uint64(2), secondUser.Revision)
	assert.Equal(t, uint64(3), afterMutation.Revision)
	assert.Equal(t, uint64(4), recreated.Revision)
}

func TestExpenseRevisionOwnerBoundsMetadata(t *testing.T) {
	owner := newExpenseRevisionOwner(2)

	owner.observe("user-1")
	owner.observe("user-2")
	owner.observe("user-3")

	assert.Len(t, owner.revisions, 2)
	newRevision := owner.observe("user-1")
	assert.Equal(t, uint64(4), newRevision.Revision)
}

func TestExpenseRevisionOwnerUsesNewEpochAfterRestart(t *testing.T) {
	firstProcess := newRevisionTestService(new(mockExpenseRepository))
	secondProcess := newRevisionTestService(new(mockExpenseRepository))

	first, err := firstProcess.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	second, err := secondProcess.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.NotEqual(t, first.Epoch, second.Epoch)
	assert.Equal(t, uint64(1), first.Revision)
	assert.Equal(t, uint64(1), second.Revision)
}

func TestGetExpenseRevisionDoesNotQueryRepository(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)

	first, err := svc.GetExpenseRevision(context.Background(), "user-no-expenses")
	require.NoError(t, err)
	second, err := svc.GetExpenseRevision(context.Background(), "user-no-expenses")
	require.NoError(t, err)

	assert.Equal(t, first, second)
	repo.AssertExpectations(t)
}

type financeEvictionTestClient struct {
	*mockPeriodContextClient
	err      error
	users    []string
	wait     bool
	deadline time.Time
}

func (c *financeEvictionTestClient) EvictUserCache(ctx context.Context, userID string) error {
	c.users = append(c.users, userID)
	c.deadline, _ = ctx.Deadline()
	if c.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	return c.err
}

func newFinanceEvictionTestClient() *financeEvictionTestClient {
	return &financeEvictionTestClient{mockPeriodContextClient: newTestPeriodClient()}
}

func TestCreateExpenseNotifiesFinanceAfterLocalInvalidation(t *testing.T) {
	repo := new(mockExpenseRepository)
	finance := newFinanceEvictionTestClient()
	svc := NewExpenseServiceWithCacheAndEviction(repo, finance, &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)), defaultReadCacheConfig(), time.Second)
	repo.On("GetExpenseByIdempotencyKey", context.Background(), "user-1", validTestUUID).Return(nil, nil)
	repo.On("CreateExpense", context.Background(), mock.AnythingOfType("*model.Expense")).Return(&model.Expense{ID: "new", UserID: "user-1"}, nil)

	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	_, err = svc.CreateExpense(context.Background(), "user-1", validCreateRequest())
	require.NoError(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Revision+1, after.Revision)
	assert.Equal(t, []string{"user-1"}, finance.users)
	repo.AssertExpectations(t)
}

func TestCreateExpenseFinanceCallbackFailureDoesNotChangeCommittedOutcome(t *testing.T) {
	repo := new(mockExpenseRepository)
	finance := newFinanceEvictionTestClient()
	finance.err = errors.New("finance unavailable")
	svc := NewExpenseServiceWithCacheAndEviction(repo, finance, &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)), defaultReadCacheConfig(), time.Second)
	repo.On("GetExpenseByIdempotencyKey", context.Background(), "user-1", validTestUUID).Return(nil, nil)
	repo.On("CreateExpense", context.Background(), mock.AnythingOfType("*model.Expense")).Return(&model.Expense{ID: "new", UserID: "user-1"}, nil)

	created, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())

	require.NoError(t, err)
	assert.Equal(t, "new", created.ID)
	assert.Equal(t, []string{"user-1"}, finance.users)
	repo.AssertExpectations(t)
}

func TestNewExpenseServiceClampsFinanceEvictionTimeout(t *testing.T) {
	svc := NewExpenseServiceWithCacheAndEviction(new(mockExpenseRepository), newTestPeriodClient(), &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)), defaultReadCacheConfig(), 2*time.Second)

	assert.Equal(t, financeconfig.MaxExpenseFinanceEvictionTimeout, svc.financeEvictionTimeout)
}

func TestFinanceEvictionCallbackUsesBoundedDeadline(t *testing.T) {
	finance := newFinanceEvictionTestClient()
	finance.wait = true
	svc := NewExpenseServiceWithCacheAndEviction(new(mockExpenseRepository), finance, &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)), defaultReadCacheConfig(), 5*time.Millisecond)

	started := time.Now()
	svc.invalidateUser("user-1")

	assert.WithinDuration(t, started.Add(5*time.Millisecond), finance.deadline, 100*time.Millisecond)
	assert.Equal(t, []string{"user-1"}, finance.users)
}

func TestCreateExpenseIdempotentReplayLeavesRevisionUnchanged(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	existing := &model.Expense{ID: "existing", UserID: "user-1", Status: "active"}
	repo.On("GetExpenseByIdempotencyKey", context.Background(), "user-1", validTestUUID).Return(existing, nil)

	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	created, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())
	require.NoError(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, existing, created)
	assert.Equal(t, before, after)
	repo.AssertExpectations(t)
}

func TestCreateExpensePurgesCacheAndAdvancesRevision(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	readRequest := &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50}
	cachedExpense := &model.Expense{ID: "old", UserID: "user-1", Status: "active"}
	repo.On("GetActiveExpensesForPeriod", context.Background(), "user-1", int32(2026), int32(5), int32(1), int32(50)).Return([]*model.Expense{cachedExpense}, int64(1), nil).Twice()

	_, err := svc.GetActiveExpensesForPeriod(context.Background(), readRequest)
	require.NoError(t, err)
	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, 1, svc.readCaches.recent.Len())

	repo.On("CreateExpense", context.Background(), mock.AnythingOfType("*model.Expense")).Return(&model.Expense{ID: "new", UserID: "user-1"}, nil)
	_, err = svc.CreateExpense(context.Background(), "user-1", validCreateRequest())
	require.NoError(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Epoch, after.Epoch)
	assert.Equal(t, before.Revision+1, after.Revision)
	assert.Equal(t, 0, svc.readCaches.recent.Len())
	fresh, err := svc.GetActiveExpensesForPeriod(context.Background(), readRequest)
	require.NoError(t, err)
	assert.Equal(t, "old", fresh.Data[0].ID)
	repo.AssertExpectations(t)
}

func TestDeleteExpenseUncertainFailurePurgesCacheAndAdvancesRevision(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	readRequest := &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50}
	repo.On("GetActiveExpensesForPeriod", context.Background(), "user-1", int32(2026), int32(5), int32(1), int32(50)).Return([]*model.Expense{{ID: "old"}}, int64(1), nil)
	_, err := svc.GetActiveExpensesForPeriod(context.Background(), readRequest)
	require.NoError(t, err)
	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	repo.On("GetExpenseByID", context.Background(), "old", "user-1").Return(&model.Expense{ID: "old", UserID: "user-1", Status: "active", PeriodYear: 2026, PeriodMonth: 5}, nil)
	repo.On("DeactivateExpense", context.Background(), "old", "user-1").Return(errors.New("write outcome unknown"))
	err = svc.DeleteExpense(context.Background(), "user-1", "old")
	require.Error(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Revision+1, after.Revision)
	assert.Equal(t, 0, svc.readCaches.recent.Len())
	repo.AssertExpectations(t)
}

func TestCorrectExpenseUncertainFailurePurgesAndAdvancesRevision(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	original := &model.Expense{
		ID:                      "original",
		UserID:                  "user-1",
		Status:                  "active",
		PeriodYear:              2026,
		PeriodMonth:             5,
		TransactionCurrencyCode: "USD",
	}
	repo.On("GetExpenseByID", context.Background(), "original", "user-1").Return(original, nil)
	repo.On("CorrectExpense", context.Background(), original, mock.AnythingOfType("*model.Expense")).Return(nil, errors.New("partial correction outcome unknown"))

	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	_, err = svc.CorrectExpense(context.Background(), "user-1", "original", validCorrectReq())
	require.Error(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Revision+1, after.Revision)
	repo.AssertExpectations(t)
}

func TestCreateProRataInstallmentFirstAndLaterMutationsAdvanceRevision(t *testing.T) {
	for _, index := range []int32{1, 2} {
		t.Run(fmt.Sprintf("installment-%d", index), func(t *testing.T) {
			repo := new(mockExpenseRepository)
			fxClient := new(mockFxClient)
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			clock := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
			svc := NewExpenseService(repo, newTestPeriodClient(), fxClient, clock, logger)
			fxClient.On("ConvertWithSnapshot", mock.Anything, mock.Anything).Return(&FxConvertResponse{
				ConvertedAmount: 3334,
				ExchangeRate:    "1",
				RateTimestamp:   "2026-05-15T10:00:00Z",
				Source:          exchangesource.OpenExchangeRates,
				ExpiresAt:       "2026-05-15T13:00:00Z",
			}, nil)
			repo.On("CreateExpense", context.Background(), mock.AnythingOfType("*model.Expense")).Return(&model.Expense{ID: "installment", UserID: "user-1"}, nil)

			req := validProRataInstallmentRequest()
			req.ProRataIndex = index
			before, err := svc.GetExpenseRevision(context.Background(), "user-1")
			require.NoError(t, err)
			_, err = svc.CreateProRataInstallment(context.Background(), req)
			require.NoError(t, err)
			after, err := svc.GetExpenseRevision(context.Background(), "user-1")
			require.NoError(t, err)

			assert.Equal(t, before.Revision+1, after.Revision)
			repo.AssertExpectations(t)
		})
	}
}

func TestAnonymizeFailurePurgesAndAdvancesRevision(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	repo.On("AnonymizeAllUserExpenses", context.Background(), "user-1").Return(errors.New("anonymization outcome unknown"))

	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	err = svc.AnonymizeAllUserExpenses(context.Background(), "user-1")
	require.Error(t, err)
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Revision+1, after.Revision)
	repo.AssertExpectations(t)
}

func TestAnonymizeRetiresRevisionBeforeRecreation(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	before, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)
	repo.On("AnonymizeAllUserExpenses", context.Background(), "user-1").Return(nil)

	require.NoError(t, svc.AnonymizeAllUserExpenses(context.Background(), "user-1"))
	after, err := svc.GetExpenseRevision(context.Background(), "user-1")
	require.NoError(t, err)

	assert.Equal(t, before.Epoch, after.Epoch)
	assert.Greater(t, after.Revision, before.Revision)
	repo.AssertExpectations(t)
}

func TestAnonymizePurgesAllUserReadCaches(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	recentRequest := &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50}
	completeRequest := &model.GetActiveExpensesForPeriodPageRequest{UserID: "user-1", Year: 2026, Month: 5, PageSize: 50}
	suggestionRequest := &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 10}
	repo.On("GetActiveExpensesForPeriod", context.Background(), "user-1", int32(2026), int32(5), int32(1), int32(50)).Return([]*model.Expense{{ID: "recent"}}, int64(1), nil).Twice()
	repo.On("GetActiveExpensesByPeriodAfter", context.Background(), "user-1", int32(2026), int32(5), repository.ActivePeriodCursor{}, int32(50)).Return([]*model.Expense{{ID: "complete"}}, repository.ActivePeriodCursor{}, false, nil).Twice()
	repo.On("GetActiveExpenseSuggestionInputs", context.Background(), "user-1").Return([]*model.ExpenseSuggestionInput{{ID: "suggestion", Name: "Coffee", CreatedAt: "2026-05-03T00:00:00Z"}}, nil).Twice()
	repo.On("AnonymizeAllUserExpenses", context.Background(), "user-1").Return(nil)

	_, err := svc.GetActiveExpensesForPeriod(context.Background(), recentRequest)
	require.NoError(t, err)
	_, err = svc.GetActiveExpensesForPeriodPage(context.Background(), completeRequest)
	require.NoError(t, err)
	_, err = svc.GetExpenseSuggestions(context.Background(), suggestionRequest)
	require.NoError(t, err)

	require.NoError(t, svc.AnonymizeAllUserExpenses(context.Background(), "user-1"))

	recent, err := svc.GetActiveExpensesForPeriod(context.Background(), recentRequest)
	require.NoError(t, err)
	complete, err := svc.GetActiveExpensesForPeriodPage(context.Background(), completeRequest)
	require.NoError(t, err)
	suggestions, err := svc.GetExpenseSuggestions(context.Background(), suggestionRequest)
	require.NoError(t, err)

	assert.Equal(t, "recent", recent.Data[0].ID)
	assert.Equal(t, "complete", complete.Data[0].ID)
	assert.Equal(t, "Coffee", suggestions.Data[0].Name)
	repo.AssertExpectations(t)
}

func TestPurgeFencesOrdinaryAndForcedServiceReads(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newRevisionTestService(repo)
	request := &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	repo.On("GetActiveExpensesForPeriod", context.Background(), "user-1", int32(2026), int32(5), int32(1), int32(50)).Return([]*model.Expense{{ID: "old"}}, int64(1), nil).Run(func(mock.Arguments) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
	})

	ordinaryDone := make(chan error, 1)
	go func() {
		_, err := svc.GetActiveExpensesForPeriod(context.Background(), request)
		ordinaryDone <- err
	}()
	<-started
	svc.invalidateUser("user-1")

	forcedDone := make(chan error, 1)
	go func() {
		forcedRequest := *request
		forcedRequest.BypassCache = true
		_, err := svc.GetActiveExpensesForPeriod(context.Background(), &forcedRequest)
		forcedDone <- err
	}()
	require.Eventually(t, func() bool { return calls.Load() == 2 }, time.Second, time.Millisecond)
	close(release)
	require.NoError(t, <-ordinaryDone)
	require.NoError(t, <-forcedDone)

	_, err := svc.GetActiveExpensesForPeriod(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

func newRevisionTestService(repo *mockExpenseRepository) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	clock := func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }
	return NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, clock, logger)
}
