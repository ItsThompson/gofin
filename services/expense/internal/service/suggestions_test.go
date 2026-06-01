package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func TestGetExpenseSuggestions_AggregatesActiveInputsAndPaginates(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo).WithClock(func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Groceries", Amount: 1000, Currency: "USD", ExpenseType: "essentials", TagID: "tag-old", CreatedAt: "2026-05-20T10:00:00Z"},
		{ID: "exp-2", Name: "Groceries", Amount: 1250, Currency: "USD", ExpenseType: "essentials", TagID: "tag-new", CreatedAt: "2026-05-31T10:00:00Z"},
		{ID: "exp-3", Name: "Coffee", Amount: 500, Currency: "USD", ExpenseType: "desires", TagID: "tag-coffee", CreatedAt: "2026-05-01T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 1})

	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(1), result.PageSize)
	assert.True(t, result.HasMore)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "Groceries", result.Data[0].Name)
	assert.Equal(t, int64(1250), result.Data[0].Amount)
	assert.Equal(t, "tag-new", result.Data[0].TagID)
	assert.Equal(t, int32(2), result.Data[0].Frequency)
	assert.Equal(t, "last_7_days", result.Data[0].RecencyBucket)
	assert.Equal(t, float64(8), result.Data[0].FrecencyScore)
}

func TestGetExpenseSuggestions_CountsProRataGroupOnce(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo).WithClock(func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Insurance", Amount: 1000, Currency: "USD", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-01T10:00:00Z", IsProRata: true, ProRataGroup: "group-1"},
		{ID: "exp-2", Name: "Insurance", Amount: 1000, Currency: "USD", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-02T10:00:00Z", IsProRata: true, ProRataGroup: "group-1"},
		{ID: "exp-3", Name: "Insurance", Amount: 1000, Currency: "USD", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-03T10:00:00Z", IsProRata: true},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), result.Data[0].Frequency)
}

func TestGetExpenseSuggestions_ValidationAndRepositoryFailure(t *testing.T) {
	tests := []struct {
		name string
		req  *model.ExpenseSuggestionRequest
	}{
		{name: "missing user", req: &model.ExpenseSuggestionRequest{Page: 1, PageSize: 50}},
		{name: "zero page", req: &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 0, PageSize: 50}},
		{name: "oversized page size", req: &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 101}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(new(mockExpenseRepository))
			_, err := svc.GetExpenseSuggestions(context.Background(), tt.req)
			require.Error(t, err)
			var svcErr *ServiceError
			require.ErrorAs(t, err, &svcErr)
			assert.Equal(t, model.ErrValidationError, svcErr.Code)
		})
	}

	repo := new(mockExpenseRepository)
	svc := newTestService(repo)
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return(nil, fmt.Errorf("immudb unavailable"))

	_, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting active expense suggestion inputs")
}
