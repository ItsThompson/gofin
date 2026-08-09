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

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
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

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
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

// panickingStreamRepo panics inside the keyset page fetch, which is where the
// producer goroutine does all of its work. All other repository methods are
// inherited unused.
type panickingStreamRepo struct {
	mockExpenseRepository
}

func (r *panickingStreamRepo) GetExpensesByUserAfter(context.Context, string, repository.ExpenseCursor, int32) ([]*model.Expense, repository.ExpenseCursor, bool, error) {
	panic("page fetch exploded")
}

// TestStreamAllUserExpenses_ProducerPanicTerminatesTheStream covers the reach
// gap the gRPC stream interceptor cannot close: the producer runs on its own
// goroutine, so a panic there is unrecoverable from the handler. Without the
// producer's own guard this test crashes the process; without the synthesized
// errc send it hangs on <-errc until the guard timeout.
func TestStreamAllUserExpenses_ProducerPanicTerminatesTheStream(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	svc := NewExpenseService(&panickingStreamRepo{}, time.Now, logger)

	sendCount := 0
	err := collectStream(t, svc, "user-1", 10, func(*model.Expense) error {
		sendCount++
		return nil
	})

	require.Error(t, err, "a producer panic must surface as a stream error, not a hang")
	assert.Zero(t, sendCount)

	records, err := logs.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1, "a recovered panic must produce exactly one error-level record")
	assert.Equal(t, "ERROR", records[0]["level"])
	assert.Equal(t, "recovered panic in expense page producer", records[0]["msg"])
	assert.Equal(t, "panic: page fetch exploded", records[0]["panic"])
	assert.Equal(t, "user-1", records[0]["user_id"])
	// The panicking frame, not debug.Stack's own first frame: a stack holding only
	// recovery machinery is useless and must fail here.
	assert.Contains(t, records[0]["stack"], "panickingStreamRepo")
}

// countingStreamRepo answers GetExpensesByUserAfter with a fixed page and an
// always-more cursor (up to a large finite guard), counting how many pages the
// producer fetched. All other repository methods are inherited unused.
type countingStreamRepo struct {
	mockExpenseRepository
	page  []*model.Expense
	next  repository.ExpenseCursor
	pages atomic.Int64
}

func (r *countingStreamRepo) GetExpensesByUserAfter(_ context.Context, _ string, _ repository.ExpenseCursor, _ int32) ([]*model.Expense, repository.ExpenseCursor, bool, error) {
	n := r.pages.Add(1)
	// hasMore stays true until a large finite guard so a correctly back-pressured
	// producer blocks on the full buffer, while a regressed unbounded producer
	// still terminates (and trips the assertion) instead of looping forever.
	return r.page, r.next, n < 1000, nil
}

func TestStreamAllUserExpenses_ProducerLookAheadBoundedToOnePage(t *testing.T) {
	// When the consumer stops draining, the producer must block once the
	// pageSize-buffered rows channel fills, retaining O(pageSize) rows rather
	// than eagerly fetching the whole history. A regression that unbounds the
	// buffer (or drops back-pressure) would fetch far more pages before blocking.
	const pageSize = int32(10)
	page := make([]*model.Expense, pageSize)
	for i := range page {
		page[i] = streamExpense(fmt.Sprintf("exp-%d", i), "2026-05-01T00:00:00Z")
	}
	repo := &countingStreamRepo{
		page: page,
		next: repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:00Z", ID: "exp-9"},
	}
	svc := NewExpenseService(repo, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstRow := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		received := 0
		done <- svc.StreamAllUserExpenses(ctx, "user-1", pageSize, func(*model.Expense) error {
			received++
			if received == 1 {
				close(firstRow)
				<-release // block after the first row so the buffer fills up
			}
			return nil
		})
	}()

	<-firstRow
	// Let the producer reach its blocked steady state (buffer full). The
	// assertion is an upper bound, so an over-short settle only lowers the
	// observed count; it never yields a false failure.
	time.Sleep(50 * time.Millisecond)
	pagesFetched := repo.pages.Load()

	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAllUserExpenses did not return after cancellation")
	}

	assert.LessOrEqual(t, pagesFetched, int64(3),
		"producer must block after buffering ~one page, not drain the full history (fetched %d pages)", pagesFetched)
}
