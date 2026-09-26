package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/cache"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/metrics"
)

func newCachedTestService(repo *mockExpenseRepository, now *time.Time, config cache.Config) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseServiceWithCache(repo, newTestPeriodClient(), &stubFxClient{}, func() time.Time { return *now }, logger, config)
}

func TestGetActiveExpensesForPeriod_CachesCopySafeResultsAndIsolatesUsers(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	stored := []*model.Expense{{ID: "expense-1", UserID: "user-1", Name: "Original", Status: "active"}}
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return(stored, int64(1), nil).Once()
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-2", int32(2026), int32(9), int32(1), int32(50)).Return([]*model.Expense{{ID: "expense-2", UserID: "user-2"}}, int64(1), nil).Once()

	first, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	first.Data[0].Name = "mutated"
	second, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "Original", second.Data[0].Name)

	otherUser, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-2", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "user-2", otherUser.Data[0].UserID)
	repo.AssertExpectations(t)
}

func TestGetActiveExpensesForPeriodPage_CachesCopySafeResults(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	repo.On("GetActiveExpensesByPeriodAfter", mock.Anything, "user-1", int32(2026), int32(9), repository.ActivePeriodCursor{}, int32(50)).Return([]*model.Expense{{ID: "expense-1", Name: "Original"}}, repository.ActivePeriodCursor{ExpenseDate: "2026-09-01", CreatedAt: "2026-09-01T10:00:00Z", ID: "expense-1"}, false, nil).Once()
	request := &model.GetActiveExpensesForPeriodPageRequest{UserID: "user-1", Year: 2026, Month: 9, PageSize: 50}

	first, err := svc.GetActiveExpensesForPeriodPage(context.Background(), request)
	require.NoError(t, err)
	first.Data[0].Name = "mutated"
	second, err := svc.GetActiveExpensesForPeriodPage(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "Original", second.Data[0].Name)
	assert.False(t, second.HasMore)
	repo.AssertExpectations(t)
}

func TestGetActiveExpensesForPeriod_ExpiryAndBypassReadSourceDirectly(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := new(mockExpenseRepository)
	config := cache.DefaultConfig()
	config.MaxAge = time.Hour
	svc := newCachedTestService(repo, &now, config)
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return([]*model.Expense{{ID: "first"}}, int64(1), nil).Once()
	first, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "first", first.Data[0].ID)

	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return([]*model.Expense{{ID: "bypass"}}, int64(1), nil).Once()
	bypassed, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50, BypassCache: true})
	require.NoError(t, err)
	assert.Equal(t, "bypass", bypassed.Data[0].ID)

	now = now.Add(2 * time.Hour)
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return([]*model.Expense{{ID: "expired"}}, int64(1), nil).Once()
	expired, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "expired", expired.Data[0].ID)
	repo.AssertExpectations(t)
}

func TestGetExpenseSuggestions_CachesEmptyInputs(t *testing.T) {
	now := time.Now()
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{}, nil).Once()
	request := &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50}

	first, err := svc.GetExpenseSuggestions(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, first.Data)
	second, err := svc.GetExpenseSuggestions(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, second.Data)
	repo.AssertExpectations(t)
}

func TestGetExpenseSuggestions_CachesInputsButRanksWithCurrentClock(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{{ID: "input-1", Name: "Coffee", CreatedAt: "2026-09-26T11:00:00Z"}}, nil).Once()

	first, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "today", first.Data[0].RecencyBucket)
	now = now.Add(24 * time.Hour)
	second, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, "last_7_days", second.Data[0].RecencyBucket)
	repo.AssertExpectations(t)
}

func TestJSONSizeMarksUnencodableValuesNonCacheable(t *testing.T) {
	size, cacheable := jsonSize(func() {})
	assert.False(t, cacheable)
	assert.Zero(t, size)
}

func TestReadCaches_DoNotStoreSourceErrorsOrEmptyValidation(t *testing.T) {
	now := time.Now()
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return(nil, int64(0), fmt.Errorf("source failed")).Twice()
	request := &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50}
	_, firstErr := svc.GetActiveExpensesForPeriod(context.Background(), request)
	_, secondErr := svc.GetActiveExpensesForPeriod(context.Background(), request)
	assert.Error(t, firstErr)
	assert.Error(t, secondErr)

	_, validationErr := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 13})
	require.Error(t, validationErr)
	repo.AssertExpectations(t)
}

func TestReadCacheMetricsUseFiniteOperationNames(t *testing.T) {
	now := time.Now()
	repo := new(mockExpenseRepository)
	svc := newCachedTestService(repo, &now, cache.DefaultConfig())
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(9), int32(1), int32(50)).Return([]*model.Expense{}, int64(0), nil).Once()
	before := testutil.ToFloat64(metrics.ExpenseReadCacheEventsTotal.WithLabelValues(expenseReadOperationRecent, "miss"))
	_, err := svc.GetActiveExpensesForPeriod(context.Background(), &model.GetExpensesRequest{UserID: "user-1", Year: 2026, Month: 9, Page: 1, PageSize: 50})
	require.NoError(t, err)
	after := testutil.ToFloat64(metrics.ExpenseReadCacheEventsTotal.WithLabelValues(expenseReadOperationRecent, "miss"))
	assert.Equal(t, float64(1), after-before)
	repo.AssertExpectations(t)
}
