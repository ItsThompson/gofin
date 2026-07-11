package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

func newTestGRPCHandler(repo *mockExpenseRepository) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, logger)
	return NewGRPCHandler(expenseSvc, logger)
}

// TestGRPC_RemovedReadRPCsAreNotRegistered locks in C2: the GetCorrectionHistory
// and GetProRataGroup RPCs hardcoded an empty ("") user scope and had no
// consumer, so they were removed from the proto and the generated service
// descriptor. The correction-history and pro-rata reads are served over REST.
// This guards against re-introducing an unscoped read RPC.
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
