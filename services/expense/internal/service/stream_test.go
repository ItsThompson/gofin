package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

func streamExpense(id, createdAt string) *model.Expense {
	return &model.Expense{
		ID:          id,
		UserID:      "user-1",
		Name:        "Expense " + id,
		Amount:      1000,
		Currency:    "USD",
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-01",
		PeriodYear:  2026,
		PeriodMonth: 5,
		Status:      "active",
		CreatedAt:   createdAt,
	}
}

// collectStream runs StreamAllUserExpenses on a fresh goroutine with a guard
// timeout so a producer that fails to stop surfaces as a test failure rather
// than a hang. sink is invoked for every streamed row.
func collectStream(t *testing.T, svc *ExpenseService, userID string, pageSize int32, sink func(*model.Expense) error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- svc.StreamAllUserExpenses(context.Background(), userID, pageSize, sink)
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAllUserExpenses did not return: producer failed to stop")
		return nil
	}
}

func TestStreamAllUserExpenses_HappyPathStreamsAllRowsInOrder(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	page1 := []*model.Expense{streamExpense("exp-1", "2026-05-01T00:00:00Z"), streamExpense("exp-2", "2026-05-01T00:00:01Z")}
	page2 := []*model.Expense{streamExpense("exp-3", "2026-05-01T00:00:02Z")}
	cursor1 := repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-2"}
	cursor2 := repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:02Z", ID: "exp-3"}

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(2)).Return(page1, cursor1, true, nil)
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", cursor1, int32(2)).Return(page2, cursor2, false, nil)

	var got []string
	err := collectStream(t, svc, "user-1", 2, func(e *model.Expense) error {
		got = append(got, e.ID)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"exp-1", "exp-2", "exp-3"}, got)
	repo.AssertExpectations(t)
}

func TestStreamAllUserExpenses_EmptyResultCleanEOF(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-empty", repository.ExpenseCursor{}, int32(50)).
		Return([]*model.Expense{}, repository.ExpenseCursor{}, false, nil)

	sendCount := 0
	err := collectStream(t, svc, "user-empty", 50, func(*model.Expense) error {
		sendCount++
		return nil
	})

	require.NoError(t, err)
	assert.Zero(t, sendCount, "empty history must stream zero rows and terminate cleanly")
	repo.AssertExpectations(t)
}

func TestStreamAllUserExpenses_MidStreamCancellationStopsProducer(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	// hasMore stays true forever: without cancellation the walk never ends, so a
	// clean return proves the producer honored cancellation and stopped.
	page := []*model.Expense{streamExpense("exp-1", "2026-05-01T00:00:00Z"), streamExpense("exp-2", "2026-05-01T00:00:01Z")}
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", mock.Anything, mock.Anything).
		Return(page, repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-2"}, true, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sendCount := 0
	done := make(chan error, 1)
	go func() {
		done <- svc.StreamAllUserExpenses(ctx, "user-1", 2, func(*model.Expense) error {
			sendCount++
			cancel() // cancel mid-stream after the first row
			return nil
		})
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, sendCount, "consumer must stop sending once the context is cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAllUserExpenses did not return after cancellation: producer failed to stop")
	}
}

func TestStreamAllUserExpenses_SendErrorStopsWalk(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	page := []*model.Expense{streamExpense("exp-1", "2026-05-01T00:00:00Z"), streamExpense("exp-2", "2026-05-01T00:00:01Z")}
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", mock.Anything, mock.Anything).
		Return(page, repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-2"}, true, nil)

	sendErr := errors.New("stream send failed")
	sendCount := 0
	err := collectStream(t, svc, "user-1", 2, func(*model.Expense) error {
		sendCount++
		return sendErr
	})

	require.ErrorIs(t, err, sendErr)
	assert.Equal(t, 1, sendCount)
}

func TestStreamAllUserExpenses_RepoErrorPropagates(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(10)).
		Return(nil, repository.ExpenseCursor{}, false, errors.New("immudb unavailable"))

	sendCount := 0
	err := collectStream(t, svc, "user-1", 10, func(*model.Expense) error {
		sendCount++
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "immudb unavailable")
	assert.Zero(t, sendCount)
	repo.AssertExpectations(t)
}

func TestStreamAllUserExpenses_ValidationErrorForEmptyUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	err := svc.StreamAllUserExpenses(context.Background(), "", 10, func(*model.Expense) error { return nil })

	var svcErr *ServiceError
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrValidationError, svcErr.Code)
}

func TestStreamAllUserExpenses_DefaultsPageSizeWhenNonPositive(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, repository.DefaultStreamPageSize).
		Return([]*model.Expense{}, repository.ExpenseCursor{}, false, nil)

	err := collectStream(t, svc, "user-1", 0, func(*model.Expense) error { return nil })

	require.NoError(t, err)
	repo.AssertExpectations(t)
}
