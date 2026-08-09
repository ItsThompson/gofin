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

// mapServiceError's default branch returns codes.Internal for any typed code the
// switch does not name, so a client error can reach the report path there. It must
// not be billed: a new conflict code or a 401 from this service is client input,
// not a service defect. Unreachable from today's service layer, and the point is
// that it stays free rather than staying unreachable.
func TestGRPC_AnUnmappedClientError_YieldsNoEvent(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, logs := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.Conflict("EXPENSE_LOCKED", "Expense is locked"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetExpense(ctx, &pb.GetExpenseRequest{UserId: "user-1", Id: "exp-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "the wire mapping is unchanged")
	assert.Equal(t, "Expense is locked", st.Message())

	assert.Empty(t, transport.Events(), "client input must not consume error quota")
	assert.Empty(t, errorRecords(t, logs))
}

// The same branch still reports a typed 5xx, which is the half that must not be
// lost while closing the 4xx exposure.
func TestGRPC_ATypedServerError_IsReported(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, _ := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.Internal("Expense store unavailable"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetExpense(ctx, &pb.GetExpenseRequest{UserId: "user-1", Id: "exp-1"})
	require.Error(t, err)

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "expense.get", events[0].Tags["operation"])
	assert.Equal(t, apierr.CodeInternal, events[0].Contexts["gofin"]["error_code"])
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
