package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func TestGetExpenseSuggestionsHandler_RequiresUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, http.MethodGet, "/api/expenses/suggestions", "", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var response apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, apierr.CodeUnauthorized, response.Code)
}

func TestGetExpenseSuggestionsHandler_DefaultsPaginationAndReturnsShape(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return([]*model.ExpenseSuggestionInput{
		{ID: "exp-1", Name: "Groceries", Amount: 2500, Currency: "USD", TransactionAmount: 2500, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "tag-food", CreatedAt: "2026-05-31T10:00:00Z"},
	}, nil)

	w := doJSONWithUserID(r, http.MethodGet, "/api/expenses/suggestions", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var response model.ExpenseSuggestionListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, int32(1), response.Page)
	assert.Equal(t, int32(50), response.PageSize)
	assert.Equal(t, int64(1), response.Total)
	assert.False(t, response.HasMore)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "Groceries", response.Data[0].Name)
	assert.Equal(t, int64(2500), response.Data[0].Amount)
	assert.Equal(t, int64(2500), response.Data[0].TransactionAmount)
	assert.Equal(t, "USD", response.Data[0].TransactionCurrency)
}

func TestGetExpenseSuggestionsHandler_RejectsInvalidPagination(t *testing.T) {
	tests := []string{
		"/api/expenses/suggestions?page=abc",
		"/api/expenses/suggestions?page=0",
		"/api/expenses/suggestions?page=-1",
		"/api/expenses/suggestions?pageSize=abc",
		"/api/expenses/suggestions?pageSize=0",
		"/api/expenses/suggestions?pageSize=-1",
		"/api/expenses/suggestions?pageSize=101",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			r := setupTestRouter(repo)

			w := doJSONWithUserID(r, http.MethodGet, path, "user-1", nil)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var response apierr.APIError
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, apierr.CodeValidation, response.Code)
		})
	}
}

func TestGetExpenseSuggestionsHandler_RepositoryFailureReturnsInternalServerError(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	repo.On("GetActiveExpenseSuggestionInputs", mock.Anything, "user-1").Return(nil, fmt.Errorf("immudb unavailable"))

	w := doJSONWithUserID(r, http.MethodGet, "/api/expenses/suggestions?page=1&pageSize=50", "user-1", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, apierr.CodeInternal, response.Code)
}
