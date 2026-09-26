package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
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

func newRevisionTestService(repo *mockExpenseRepository) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	clock := func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }
	return NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, clock, logger)
}
