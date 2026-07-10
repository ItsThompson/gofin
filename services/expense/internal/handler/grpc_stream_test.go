package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// fakeStreamServer implements pb.ExpenseService_StreamAllUserExpensesServer
// (grpc.ServerStreamingServer[pb.ExpenseData]) for handler tests. The embedded
// nil grpc.ServerStream supplies the unused stream methods; only Context and
// Send are exercised.
type fakeStreamServer struct {
	grpc.ServerStream
	ctx     context.Context
	sent    []*pb.ExpenseData
	sendErr error
	failAt  int // 1-based row index whose Send returns sendErr; 0 = never fail
}

func (f *fakeStreamServer) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *fakeStreamServer) Send(e *pb.ExpenseData) error {
	if f.failAt > 0 && len(f.sent)+1 == f.failAt {
		return f.sendErr
	}
	f.sent = append(f.sent, e)
	return nil
}

func streamRow(id, createdAt string) *model.Expense {
	return &model.Expense{
		ID:          id,
		UserID:      "user-1",
		Name:        "Expense " + id,
		Amount:      2500,
		Currency:    "USD",
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-01",
		PeriodYear:  2026,
		PeriodMonth: 5,
		Status:      "active",
		CreatedAt:   createdAt,
	}
}

func TestGRPC_StreamAllUserExpenses_SendsAllRowsAsProto(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	page := []*model.Expense{streamRow("exp-1", "2026-05-01T00:00:00Z"), streamRow("exp-2", "2026-05-01T00:00:01Z")}
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(2)).
		Return(page, repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-2"}, false, nil)

	stream := &fakeStreamServer{}
	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: "user-1", PageSize: 2}, stream)

	require.NoError(t, err)
	require.Len(t, stream.sent, 2)
	assert.Equal(t, "exp-1", stream.sent[0].GetId())
	assert.Equal(t, "exp-2", stream.sent[1].GetId())
	assert.Equal(t, int64(2500), stream.sent[0].GetAmount())
	assert.Equal(t, "2026-05-01T00:00:01Z", stream.sent[1].GetCreatedAt())
	repo.AssertExpectations(t)
}

func TestGRPC_StreamAllUserExpenses_EmptyUserIDReturnsInvalidArgument(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	stream := &fakeStreamServer{}
	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: ""}, stream)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Empty(t, stream.sent)
}

func TestGRPC_StreamAllUserExpenses_PropagatesSendError(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	page := []*model.Expense{streamRow("exp-1", "2026-05-01T00:00:00Z"), streamRow("exp-2", "2026-05-01T00:00:01Z")}
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", mock.Anything, mock.Anything).
		Return(page, repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-2"}, false, nil)

	sendErr := errors.New("client disconnected")
	stream := &fakeStreamServer{sendErr: sendErr, failAt: 1}
	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: "user-1", PageSize: 2}, stream)

	require.ErrorIs(t, err, sendErr)
	assert.Empty(t, stream.sent)
}
