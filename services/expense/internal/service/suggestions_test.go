package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func TestGetExpenseSuggestions_AggregatesActiveInputsAndPaginates(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Groceries", TransactionAmount: 1000, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "tag-old", CreatedAt: "2026-05-20T10:00:00Z"},
		{ID: "exp-2", Name: "Groceries", TransactionAmount: 1250, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "tag-new", CreatedAt: "2026-05-31T10:00:00Z"},
		{ID: "exp-3", Name: "Coffee", TransactionAmount: 500, TransactionCurrency: "USD", ExpenseType: "desires", TagID: "tag-coffee", CreatedAt: "2026-05-01T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 1})

	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(1), result.PageSize)
	assert.True(t, result.HasMore)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "Groceries", result.Data[0].Name)
	assert.Equal(t, int64(1250), result.Data[0].TransactionAmount)
	assert.Equal(t, "USD", result.Data[0].TransactionCurrency)
	assert.Equal(t, "tag-new", result.Data[0].TagID)
	assert.Equal(t, int32(2), result.Data[0].Frequency)
	assert.Equal(t, "last_7_days", result.Data[0].RecencyBucket)
	assert.Equal(t, float64(8), result.Data[0].FrecencyScore)
}

func TestGetExpenseSuggestions_PreservesForeignTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Hotel", TransactionAmount: 15000, TransactionCurrency: "EUR", ExpenseType: "desires", TagID: "tag-travel", CreatedAt: "2026-05-28T10:00:00Z"},
		{ID: "exp-2", Name: "Hotel", TransactionAmount: 16000, TransactionCurrency: "EUR", ExpenseType: "desires", TagID: "tag-travel", CreatedAt: "2026-05-30T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	// The latest active matching expense determines the suggestion values.
	assert.Equal(t, int64(16000), result.Data[0].TransactionAmount)
	assert.Equal(t, "EUR", result.Data[0].TransactionCurrency)
}

func TestGetExpenseSuggestions_CountsProRataGroupOnce(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Insurance", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-01T10:00:00Z", IsProRata: true, ProRataGroup: "group-1"},
		{ID: "exp-2", Name: "Insurance", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-02T10:00:00Z", IsProRata: true, ProRataGroup: "group-1"},
		{ID: "exp-3", Name: "Insurance", ExpenseType: "essentials", TagID: "tag-ins", CreatedAt: "2026-05-03T10:00:00Z", IsProRata: true},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), result.Data[0].Frequency)
}

func TestGetExpenseSuggestions_RankingTieBreakers(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Coffee", ExpenseType: "desires", TagID: "tag-coffee", CreatedAt: "2026-05-30T10:00:00Z"},
		{ID: "exp-2", Name: "Coffee", ExpenseType: "desires", TagID: "tag-coffee", CreatedAt: "2026-05-30T11:00:00Z"},
		{ID: "exp-3", Name: "Groceries", ExpenseType: "essentials", TagID: "tag-food", CreatedAt: "2026-05-31T10:00:00Z"},
		{ID: "exp-4", Name: "Transit", ExpenseType: "essentials", TagID: "tag-transit", CreatedAt: "2026-05-29T10:00:00Z"},
		{ID: "exp-5", Name: "Apples", ExpenseType: "essentials", TagID: "tag-food", CreatedAt: "2026-05-28T10:00:00Z"},
		{ID: "exp-6", Name: "Bakery", ExpenseType: "desires", TagID: "tag-bakery", CreatedAt: "2026-05-28T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	require.Len(t, result.Data, 5)
	assert.Equal(t, []string{"Coffee", "Groceries", "Transit", "Apples", "Bakery"}, suggestionNames(result.Data))
}

func TestGetExpenseSuggestions_UsesIDTieBreakerForLatestValues(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Lunch", TransactionAmount: 1200, TransactionCurrency: "USD", ExpenseType: "desires", TagID: "tag-old", CreatedAt: "2026-05-31T10:00:00Z"},
		{ID: "exp-2", Name: "Lunch", TransactionAmount: 1500, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "tag-new", CreatedAt: "2026-05-31T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int64(1500), result.Data[0].TransactionAmount)
	assert.Equal(t, "essentials", result.Data[0].ExpenseType)
	assert.Equal(t, "tag-new", result.Data[0].TagID)
}

func TestGetExpenseSuggestions_AssignsRecencyBucketWeights(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Today", ExpenseType: "essentials", TagID: "tag-1", CreatedAt: "2026-06-01T01:00:00Z"},
		{ID: "exp-2", Name: "Last 7", ExpenseType: "essentials", TagID: "tag-2", CreatedAt: "2026-05-29T12:00:00Z"},
		{ID: "exp-3", Name: "Last 30", ExpenseType: "essentials", TagID: "tag-3", CreatedAt: "2026-05-15T12:00:00Z"},
		{ID: "exp-4", Name: "Older", ExpenseType: "essentials", TagID: "tag-4", CreatedAt: "2026-04-15T12:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	assertSuggestionBucket(t, result.Data, "Today", "today", 8)
	assertSuggestionBucket(t, result.Data, "Last 7", "last_7_days", 4)
	assertSuggestionBucket(t, result.Data, "Last 30", "last_30_days", 2)
	assertSuggestionBucket(t, result.Data, "Older", "older", 1)
}

func TestGetExpenseSuggestions_PaginatesRankedSuggestions(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	inputs := make([]*model.ExpenseSuggestionInput, 0, 51)
	for i := 0; i < 51; i++ {
		inputs = append(inputs, &model.ExpenseSuggestionInput{
			ID:          fmt.Sprintf("exp-%02d", i),
			Name:        fmt.Sprintf("Expense %02d", i),
			ExpenseType: "essentials",
			TagID:       "tag",
			CreatedAt:   "2026-05-31T10:00:00Z",
		})
	}
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return(inputs, nil)

	pageOne, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.NoError(t, err)
	assert.Len(t, pageOne.Data, 50)
	assert.Equal(t, int64(51), pageOne.Total)
	assert.True(t, pageOne.HasMore)

	pageTwo, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 2, PageSize: 50})

	require.NoError(t, err)
	assert.Len(t, pageTwo.Data, 1)
	assert.Equal(t, int64(51), pageTwo.Total)
	assert.False(t, pageTwo.HasMore)
}

func TestGetExpenseSuggestions_ReturnsEmptyPageWhenPageStartExceedsTotal(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestServiceWithClock(repo, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Groceries", ExpenseType: "essentials", TagID: "tag-food", CreatedAt: "2026-05-31T10:00:00Z"},
	}, nil)

	result, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 2, PageSize: 50})

	require.NoError(t, err)
	assert.Empty(t, result.Data)
	assert.Equal(t, int64(1), result.Total)
	assert.False(t, result.HasMore)
}

func suggestionNames(suggestions []*model.ExpenseSuggestion) []string {
	names := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		names = append(names, suggestion.Name)
	}
	return names
}

func assertSuggestionBucket(t *testing.T, suggestions []*model.ExpenseSuggestion, name string, expectedBucket string, expectedScore float64) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Name == name {
			assert.Equal(t, expectedBucket, suggestion.RecencyBucket)
			assert.Equal(t, expectedScore, suggestion.FrecencyScore)
			return
		}
	}
	require.Failf(t, "suggestion not found", "expected suggestion %q", name)
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
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
		})
	}

	repo := new(mockExpenseRepository)
	svc := newTestService(repo)
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return(nil, fmt.Errorf("immudb unavailable"))

	_, err := svc.GetExpenseSuggestions(context.Background(), &model.ExpenseSuggestionRequest{UserID: "user-1", Page: 1, PageSize: 50})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting active expense suggestion inputs")
}
