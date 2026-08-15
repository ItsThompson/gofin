package handler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

func newProRataTestHandler(repo *mockExpenseRepository, fx service.FxClient) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := service.NewExpenseService(repo, newTestPeriodClient(), fx, time.Now, logger)
	return NewGRPCHandler(svc)
}

func TestGRPC_CreateProRataInstallment_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	fx := new(mockFxClient)
	handler := newProRataTestHandler(repo, fx)

	fx.On("ConvertWithSnapshot", mock.Anything, mock.Anything).Return(&service.FxConvertResponse{
		ConvertedAmount: 3334,
		ExchangeRate:    "1",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		Source:          model.ExchangeSourceOpenExchangeRates,
		ExpiresAt:       "2026-05-15T13:00:00Z",
	}, nil)

	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Return(&model.Expense{ID: "exp-1", UserID: "user-1"}, nil)

	resp, err := handler.CreateProRataInstallment(context.Background(), &pb.CreateProRataInstallmentRequest{
		UserId: "user-1",
		PeriodContext: &pb.TrustedPeriodContext{
			PeriodId:          "period-1",
			UserId:            "user-1",
			Year:              2026,
			Month:             5,
			ReportingCurrency: "USD",
			Source:            "finance_service",
		},
		Name:                "Annual subscription",
		Amount:              3334,
		TransactionCurrency: "USD",
		ExpenseType:         "essentials",
		TagId:               "tag-1",
		ExpenseDate:         "2026-05-15",
		ProRataGroup:        "group-1",
		ProRataIndex:        1,
		ProRataTotal:        3,
		CapturedRateSnapshot: &pb.CapturedRateSnapshot{
			SnapshotVersion: 1,
			Source:          "open_exchange_rates",
			BaseCurrency:    "USD",
			RateTimestamp:   "2026-05-15T10:00:00Z",
			CapturedAt:      "2026-05-15T12:00:00Z",
			ExpiresAt:       "2026-05-15T13:00:00Z",
			RatesByCurrency: map[string]string{"USD": "1"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "exp-1", resp.GetExpense().GetId())
	repo.AssertExpectations(t)
}

func TestGRPC_CreateProRataInstallment_MissingSnapshotCoverage(t *testing.T) {
	repo := new(mockExpenseRepository)
	fx := new(mockFxClient)
	handler := newProRataTestHandler(repo, fx)

	_, err := handler.CreateProRataInstallment(context.Background(), &pb.CreateProRataInstallmentRequest{
		UserId: "user-1",
		PeriodContext: &pb.TrustedPeriodContext{
			PeriodId:          "period-1",
			UserId:            "user-1",
			Year:              2026,
			Month:             5,
			ReportingCurrency: "USD",
			Source:            "finance_service",
		},
		Name:                "Annual subscription",
		Amount:              3334,
		TransactionCurrency: "EUR",
		ExpenseType:         "essentials",
		TagId:               "tag-1",
		ExpenseDate:         "2026-05-15",
		ProRataGroup:        "group-1",
		ProRataIndex:        1,
		ProRataTotal:        3,
		CapturedRateSnapshot: &pb.CapturedRateSnapshot{
			SnapshotVersion: 1,
			Source:          "open_exchange_rates",
			BaseCurrency:    "USD",
			RateTimestamp:   "2026-05-15T10:00:00Z",
			RatesByCurrency: map[string]string{"USD": "1"},
		},
	})

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	fx.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}
