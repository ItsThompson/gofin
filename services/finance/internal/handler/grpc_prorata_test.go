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

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func setupProRataGRPCHandler(repo *mockFinanceRepository, exp *mockExpenseClient, fx *mockFxClient) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	financeSvc := service.NewFinanceServiceWithFx(repo, new(mockTxBeginner), exp, fx, func() time.Time { return now }, logger)
	return NewGRPCHandler(financeSvc)
}

func TestGRPC_CreateProRataExpense_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	exp := new(mockExpenseClient)
	fx := new(mockFxClient)
	handler := setupProRataGRPCHandler(repo, exp, fx)

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 5, ReportingCurrencyCode: "USD"}, nil)

	snapshot := &model.CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          "open_exchange_rates",
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1"},
	}
	fx.On("CaptureRateSnapshot", mock.Anything, mock.Anything).Return(snapshot, nil)
	exp.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req service.CreateProRataInstallmentInput) bool {
		return req.Currency == "USD" && req.PeriodContext.Year == 2026 && req.PeriodContext.Month == 5
	})).Return(&service.CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)
	repo.On("CreateProRataSchedule", mock.Anything, mock.Anything).
		Return(&model.ProRataSchedule{ID: "sched-1", Status: "pending"}, nil)

	resp, err := handler.CreateProRataExpense(context.Background(), &pb.CreateProRataExpenseRequest{
		UserId:              "user-1",
		Name:                "Annual subscription",
		TotalAmount:         6000,
		TransactionCurrency: "USD",
		ExpenseType:         "essentials",
		TagId:               "tag-1",
		ExpenseDate:         "2026-05-15",
		Months:              2,
		PeriodYear:          2026,
		PeriodMonth:         5,
	})

	require.NoError(t, err)
	assert.Equal(t, "pro-rata schedule created", resp.GetMessage())
	exp.AssertExpectations(t)
}

func TestGRPC_CreateProRataExpense_MissingPeriod(t *testing.T) {
	repo := new(mockFinanceRepository)
	exp := new(mockExpenseClient)
	fx := new(mockFxClient)
	handler := setupProRataGRPCHandler(repo, exp, fx)

	resp, err := handler.CreateProRataExpense(context.Background(), &pb.CreateProRataExpenseRequest{
		UserId:              "user-1",
		Name:                "Annual subscription",
		TotalAmount:         6000,
		TransactionCurrency: "USD",
		ExpenseType:         "essentials",
		TagId:               "tag-1",
		ExpenseDate:         "2026-05-15",
		Months:              2,
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	exp.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
}
