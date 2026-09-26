package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

func TestPersonalFinanceResponsesSetNoStoreOnSuccessAndError(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "period-1", UserID: "user-123", Year: 2026, Month: 5}, nil)
	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(6)).
		Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)
	success := doJSONWithUserID(r, http.MethodGet, "/api/finance/periods/current?year=2026&month=5", "user-123", nil)
	assert.Equal(t, http.StatusOK, success.Code)
	assert.Equal(t, "no-store", success.Header().Get("Cache-Control"))

	errorResponse := doJSONWithUserID(r, http.MethodGet, "/api/finance/periods/current?year=2026&month=6", "user-123", nil)
	assert.Equal(t, http.StatusNotFound, errorResponse.Code)
	assert.Equal(t, "no-store", errorResponse.Header().Get("Cache-Control"))
}
