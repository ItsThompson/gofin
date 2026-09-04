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
	fxClient := new(mockFxClient)

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceServiceWithFx(repo, txBeginner, expClient, fxClient, func() time.Time { return now }, logger)

	gin.SetMode(gin.TestMode)
	h := NewRESTHandler(financeSvc)
	r := gin.New()
	h.RegisterRoutes(r)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{
			ID: "period-1", UserID: "user-1", Year: 2026, Month: 5, ReportingCurrency: "USD",
		}, nil)

	snapshot := &model.CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          "open_exchange_rates",
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		CapturedAt:      "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.92"},
	}
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.MatchedBy(func(req service.FxCaptureRequest) bool {
		return len(req.RequiredCurrencies) == 2 && req.RequiredCurrencies[0] == "EUR" && req.RequiredCurrencies[1] == "USD"
	})).Return(snapshot, nil)

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req service.CreateProRataInstallmentInput) bool {
		return req.Currency == "EUR" &&
			req.PeriodContext.Year == 2026 &&
			req.PeriodContext.Month == 5 &&
			req.PeriodContext.ReportingCurrency == "USD" &&
			req.CapturedRateSnapshot == snapshot
	})).Return(&service.CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.TransactionCurrencyCode == "EUR" &&
			s.CreationReportingCurrency == "USD" &&
			s.CapturedRateSnapshot.RateTimestamp == snapshot.RateTimestamp
	})).Return(&model.ProRataSchedule{
		ID: "sched-1", Status: "pending",
	}, nil)

	w := doJSONWithUserID(r, "POST", "/api/finance/prorata", "user-1", map[string]interface{}{
		"name":                    "Annual subscription",
		"totalAmountInMinorUnits": 6000,
		"transactionCurrencyCode": "EUR",
		"expenseType":             "essentials",
		"tagId":                   "tag-1",
		"expenseDateIso":          "2026-05-15",
		"spreadOverMonths":        2,
		"periodYear":              2026,
		"periodMonth":             5,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ProRataResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-1", resp.Expense.ID)
	assert.Equal(t, "EUR", resp.Expense.TransactionCurrencyCode)
	expClient.AssertExpectations(t)
	fxClient.AssertExpectations(t)
}
