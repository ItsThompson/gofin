package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func TestPersonalExpenseResponsesSetNoStoreOnSuccessAndError(t *testing.T) {
	repo := new(mockExpenseRepository)
	repo.On("GetActiveExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(5), int32(1), int32(50)).
		Return([]*model.Expense{}, int64(0), nil)

	r := setupTestRouter(repo)
	success := doJSONWithUserID(r, http.MethodGet, "/api/expenses?year=2026&month=5", "user-123", nil)
	assert.Equal(t, http.StatusOK, success.Code)
	assert.Equal(t, "no-store", success.Header().Get("Cache-Control"))

	errorResponse := doJSONWithUserID(r, http.MethodGet, "/api/expenses", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, errorResponse.Code)
	assert.Equal(t, "no-store", errorResponse.Header().Get("Cache-Control"))
}
