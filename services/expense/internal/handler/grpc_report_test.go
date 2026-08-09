package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// The streaming RPC is the one site whose context does not arrive as a parameter,
// so the hub is reachable only through stream.Context(). A report built on a
// background context would find no hub and lose the request and the trace.
func TestGRPC_StreamAllUserExpenses_ReportsThroughTheStreamContext(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, _ := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(2)).
		Return(nil, repository.ExpenseCursor{}, false, apierr.Internal("Expense store unavailable"))

	transport := &errkittest.Transport{}
	stream := &fakeStreamServer{ctx: errkittest.ContextWithHub(context.Background(), transport)}

	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: "user-1", PageSize: 2}, stream)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "Expense store unavailable", st.Message())

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "expense.stream_all", events[0].Tags["operation"])
	assert.Equal(t, "expenses", events[0].Tags["domain"])
}

// A send failure means the consumer stopped reading. The only consumer is the
// export engine, which fails its own job and reports that with the job id, so a
// report here would bill a second event for one failure. It stays silent
// deliberately, and this is the assertion that keeps it that way.
func TestGRPC_StreamSendFailure_ReportsNothing(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, _ := newGRPCHandlerWithLog(t, repo)

	page := []*model.Expense{streamRow("exp-1", "2026-05-01T00:00:00Z")}
	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(1)).
		Return(page, repository.ExpenseCursor{CreatedAt: "2026-05-01T00:00:00Z", ID: "exp-1"}, false, nil)

	transport := &errkittest.Transport{}
	stream := &fakeStreamServer{
		ctx:     errkittest.ContextWithHub(context.Background(), transport),
		sendErr: errors.New("transport is closing"),
		failAt:  1,
	}

	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: "user-1", PageSize: 1}, stream)

	require.Error(t, err)
	assert.Empty(t, transport.Events())
}

// A cancelled call is a client outcome, and one event per disconnected consumer
// would be an unbounded source against a 5,000-event monthly allowance.
func TestGRPC_StreamCancelled_ReportsNothing(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, _ := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpensesByUserAfter", mock.Anything, "user-1", repository.ExpenseCursor{}, int32(1)).
		Return(nil, repository.ExpenseCursor{}, false, context.Canceled)

	transport := &errkittest.Transport{}
	stream := &fakeStreamServer{ctx: errkittest.ContextWithHub(context.Background(), transport)}

	err := handler.StreamAllUserExpenses(&pb.StreamAllUserExpensesRequest{UserId: "user-1", PageSize: 1}, stream)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Canceled, st.Code())
	assert.Empty(t, transport.Events())
}
