package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// repoFailure is the untyped repository error the internal-failure tests inject.
// It stands in for any infrastructure fault the handler cannot classify, and it
// must never reach the gRPC status message.
const repoFailure = "connection refused"

// newGRPCHandlerWithLog builds a gRPC handler whose records land in the returned
// buffer, so a test can assert on what the handler recorded.
//
// The sink is installed as slog.Default because errkit writes its record through
// the package-level logger, which every service main sets to its own.
func newGRPCHandlerWithLog(t *testing.T, repo *mockExpenseRepository) (*GRPCHandler, *bytes.Buffer) {
	t.Helper()

	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	expenseSvc := service.NewExpenseService(repo, time.Now, logger)
	return NewGRPCHandler(expenseSvc), buf
}

// errorRecords parses the buffered log output and returns the error-level
// records only, so a test can assert both their number and their content.
func errorRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["level"] == slog.LevelError.String() {
			records = append(records, record)
		}
	}
	return records
}

// TestGRPC_UnclassifiedError_LogsCauseAndKeepsItOffTheWire covers
// mapServiceError's fallthrough exit: a repository error the handler cannot
// classify must produce codes.Internal, exactly one error-level record naming
// the cause and the calling RPC, and a status message with no internal detail.
func TestGRPC_UnclassifiedError_LogsCauseAndKeepsItOffTheWire(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, logs := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(nil, errors.New(repoFailure))

	_, err := handler.GetExpense(context.Background(), &pb.GetExpenseRequest{UserId: "user-1", Id: "exp-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal error", st.Message())
	assert.NotContains(t, st.Message(), repoFailure, "the cause must not reach the caller")

	records := errorRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "unclassified service error", records[0]["msg"])
	assert.Equal(t, "GetExpense", records[0]["method"])
	assert.Equal(t, "expense.get", records[0]["operation"])
	assert.Equal(t, "user-1", records[0]["user_id"])
	assert.Equal(t, "getting expense: "+repoFailure, records[0]["error"])
}

// TestGRPC_TypedInternalError_LogsCause covers mapServiceError's other
// codes.Internal exit, the one that forwards a typed *apierr.Error message to
// the caller. The wire message is deliberately unchanged, so the record is what
// carries the wrapped cause.
func TestGRPC_TypedInternalError_LogsCause(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, logs := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.Internal("Expense store unavailable"))

	_, err := handler.GetExpense(context.Background(), &pb.GetExpenseRequest{UserId: "user-1", Id: "exp-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "Expense store unavailable", st.Message())

	records := errorRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "internal service error", records[0]["msg"])
	assert.Equal(t, "GetExpense", records[0]["method"])
	assert.Equal(t, "expense.get", records[0]["operation"])
	assert.Equal(t, apierr.CodeInternal, records[0]["error_code"])
	assert.Equal(t, "getting expense: Expense store unavailable", records[0]["error"])
}

// TestGRPC_AnonymizeAllUserExpenses_LogsCause covers the one codes.Internal exit
// that does not go through mapServiceError.
func TestGRPC_AnonymizeAllUserExpenses_LogsCause(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, logs := newGRPCHandlerWithLog(t, repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(errors.New(repoFailure))

	_, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{UserId: "user-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "failed to anonymize expenses", st.Message())
	assert.NotContains(t, st.Message(), repoFailure, "the cause must not reach the caller")

	records := errorRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "failed to anonymize expenses", records[0]["msg"])
	assert.Equal(t, "AnonymizeAllUserExpenses", records[0]["method"])
	assert.Equal(t, "expense.anonymize", records[0]["operation"])
	assert.Equal(t, "user-1", records[0]["user_id"])
	assert.Equal(t, "anonymizing user expenses: "+repoFailure, records[0]["error"])
}

// TestGRPC_ClassifiedError_IsNotLoggedAsInternal asserts mapServiceError only
// records the errors it cannot classify: a typed NOT_FOUND is an expected client
// outcome, so it maps to codes.NotFound and emits no error record.
func TestGRPC_ClassifiedError_IsNotLoggedAsInternal(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler, logs := newGRPCHandlerWithLog(t, repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.NotFound("expense exp-1 not found"))

	_, err := handler.GetExpense(context.Background(), &pb.GetExpenseRequest{UserId: "user-1", Id: "exp-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Empty(t, errorRecords(t, logs))
}
