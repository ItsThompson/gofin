package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

func newTestGRPCHandler(repo *mockExpenseRepository) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, newTestPeriodClient(), time.Now, logger)
	return NewGRPCHandler(expenseSvc)
}

// TestGRPC_RemovedReadRPCsAreNotRegistered asserts GetCorrectionHistory and
// GetProRataGroup are served over REST, not gRPC. This guards against
// re-introducing an unscoped read RPC on the gRPC surface.
func TestGRPC_RemovedReadRPCsAreNotRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, m := range pb.ExpenseService_ServiceDesc.Methods {
		registered[m.MethodName] = true
	}
	for _, s := range pb.ExpenseService_ServiceDesc.Streams {
		registered[s.StreamName] = true
	}

	assert.NotContains(t, registered, "GetCorrectionHistory")
	assert.NotContains(t, registered, "GetProRataGroup")

	// The rest of the gRPC surface is unchanged.
	assert.Contains(t, registered, "CreateExpense")
	assert.Contains(t, registered, "GetExpensesForPeriod")
	assert.Contains(t, registered, "GetExpense")
	assert.Contains(t, registered, "CorrectExpense")
	assert.Contains(t, registered, "CountExpensesByTag")
	assert.Contains(t, registered, "AnonymizeAllUserExpenses")
	assert.Contains(t, registered, "StreamAllUserExpenses")
}

func TestGRPC_CreateExpense_UsesTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(expense *model.Expense) bool {
		return expense.TransactionCurrency == "EUR" && expense.Currency == "EUR"
	})).Return(&model.Expense{
		ID:                  "exp-1",
		UserID:              "user-1",
		Amount:              1200,
		TransactionCurrency: "EUR",
		Currency:            "EUR",
		Status:              "active",
	}, nil)

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:              "user-1",
		Name:                "Coffee",
		Amount:              1200,
		TransactionCurrency: "EUR",
		ExpenseType:         "desires",
		TagId:               "tag-food",
		ExpenseDate:         "2026-05-03",
		PeriodYear:          2026,
		PeriodMonth:         5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp.GetExpense())
	assert.Equal(t, "EUR", resp.GetExpense().GetTransactionCurrency())
	repo.AssertExpectations(t)
}

func TestGRPC_CreateExpense_MissingPeriodReturnsNotFoundWithYearMonth(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, time.Now, logger)
	handler := NewGRPCHandler(expenseSvc)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(6)).
		Return(nil, &apierr.Error{
			Code:    model.ErrPeriodNotFound,
			Message: "No budget period found for 2026-06",
			Status:  404,
			Fields:  map[string]string{"periodYear": "2026", "periodMonth": "6"},
		})

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:      "user-1",
		Name:        "Coffee",
		Amount:      450,
		Currency:    "USD",
		ExpenseType: "desires",
		TagId:       "tag-food",
		ExpenseDate: "2026-06-03",
		PeriodYear:  2026,
		PeriodMonth: 6,
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "2026-06")
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestGRPC_GetExpense_WrappedTypedErrorClassifies asserts a typed *apierr.Error
// that the service %w-wraps before it reaches the gRPC handler must still
// classify via errors.As (not collapse to codes.Internal).
func TestGRPC_GetExpense_WrappedTypedErrorClassifies(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.NotFound("expense exp-1 not found"))

	resp, err := handler.GetExpense(context.Background(), &pb.GetExpenseRequest{
		UserId: "user-1",
		Id:     "exp-1",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// --- AnonymizeAllUserExpenses gRPC handler tests ---

func TestGRPC_AnonymizeAllUserExpenses_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	repo.AssertExpectations(t)
}

func TestGRPC_AnonymizeAllUserExpenses_EmptyUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "",
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id is required")
}

func TestGRPC_AnonymizeAllUserExpenses_Idempotent(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	// Calling for already-redacted data: repo returns nil (success)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-already-redacted").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-already-redacted",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPC_AnonymizeAllUserExpenses_NoExpenses(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	// User has no expenses: repo returns nil (0 rows updated)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-no-data").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-no-data",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPC_AnonymizeAllUserExpenses_DatabaseFailure(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").
		Return(fmt.Errorf("database connection timeout"))

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-1",
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to anonymize expenses")
}
