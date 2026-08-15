package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

func TestCreateProRataExpenseHandler_TransactionCurrencyOnly(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, expClient, func() time.Time { return now }, logger)

	gin.SetMode(gin.TestMode)
	h := NewRESTHandler(financeSvc)
	r := gin.New()
	h.RegisterRoutes(r)

	expClient.On("CreateExpense", mock.Anything, mock.MatchedBy(func(req service.CreateExpenseInput) bool {
		return req.Currency == "EUR" && req.IsProRata
	})).Return(&service.CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.Currency == "EUR"
	})).Return(&model.ProRataSchedule{
		ID: "sched-1", Status: "pending",
	}, nil)

	w := doJSONWithUserID(r, "POST", "/api/finance/prorata", "user-1", map[string]interface{}{
		"name":                "Annual subscription",
		"totalAmount":         6000,
		"transactionCurrency": "EUR",
		"expenseType":         "essentials",
		"tagId":               "tag-1",
		"expenseDate":         "2026-05-15",
		"months":              2,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ProRataResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-1", resp.Expense.ID)
	assert.Equal(t, "EUR", resp.Expense.Currency)
	expClient.AssertExpectations(t)
}
