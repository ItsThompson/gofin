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
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// This is the representative for every internal exit in both gRPC handlers: they
// are the same shape by design, so what is asserted here is the shape, not the
// site. The taxonomy matters because a shared reporter is the top stack frame of
// everything it reports, and the Sentry server drops the exception message from
// grouping whenever a stack is present, so the operation in the group key is what
// keeps one issue per operation instead of one issue for the service.
func TestGRPC_InternalFailure_YieldsOneEventNamingTheOperation(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, _ := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("UpdateTag", mock.Anything, "tag-1", "user-1", "Groceries").
		Return(nil, errors.New(repoFailure))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.UpdateTag(ctx, &pb.UpdateTagRequest{
		UserId: "user-1", TagId: "tag-1", Name: "Groceries",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "failed to update tag", st.Message(), "the wire message does not move")

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "finance.update_tag", events[0].Tags["operation"])
	assert.Equal(t, "budgets", events[0].Tags["domain"])
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
	assert.Equal(t, []string{"{{ default }}", "finance.update_tag/internal"}, events[0].Fingerprint)
	assert.Contains(t, events[0].Exception[len(events[0].Exception)-1].Value, repoFailure)
	assert.Equal(t, map[string]any{
		"method":  "UpdateTag",
		"user_id": "user-1",
		"tag_id":  "tag-1",
	}, events[0].Contexts["gofin"])
}

// A shared reporter with one generic operation would collapse every gRPC failure
// in the service into one group key, which is the failure mode the whole helper
// exists to prevent. Two operations must differ.
func TestGRPC_TwoOperationsGroupSeparately(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, _ := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, errors.New(repoFailure))
	repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(0), errors.New(repoFailure))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetDefaults(ctx, &pb.GetDefaultsRequest{UserId: "user-1"})
	require.Error(t, err)
	_, err = handler.ListTags(ctx, &pb.ListTagsRequest{UserId: "user-1"})
	require.Error(t, err)

	events := transport.Events()
	require.Len(t, events, 2)
	assert.NotEqual(t, events[0].Fingerprint, events[1].Fingerprint)
}

// A typed 4xx is an expected client outcome. It costs no quota, and that holds by
// construction rather than by an enumerated list: every 4xx is an *apierr.Error
// with an explicit Status, so the branch above the report handles it.
func TestGRPC_ClassifiedError_YieldsNoEvent(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, _ := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("GetTag", mock.Anything, "tag-1", "user-1").Return((*model.Tag)(nil), nil)

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.DeleteTag(ctx, &pb.DeleteTagRequest{UserId: "user-1", TagId: "tag-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
	assert.Empty(t, transport.Events())
}

// Six of the eight internal exits classify only the codes they map, so a typed 4xx
// this handler does not name reaches the report path. It must not be billed: the
// error is client input either way, and the exit's own status pairing is what a
// caller would notice. Unreachable from today's service layer, and the point is
// that it stays free rather than staying unreachable.
func TestGRPC_AnUnmappedClientError_YieldsNoEvent(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, logs := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("GetDefaults", mock.Anything, "user-1").
		Return(nil, apierr.Validation("year must be positive", nil))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetDefaults(ctx, &pb.GetDefaultsRequest{UserId: "user-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "the wire mapping is unchanged")

	assert.Empty(t, transport.Events(), "client input must not consume error quota")
	assert.Empty(t, errorRecords(t, logs))
}

// The same exit still reports a typed 5xx, which is the half that must not be lost
// while closing the 4xx exposure.
func TestGRPC_ATypedServerError_IsReported(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, _ := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("GetDefaults", mock.Anything, "user-1").
		Return(nil, apierr.Internal("Defaults store unavailable"))

	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)

	_, err := handler.GetDefaults(ctx, &pb.GetDefaultsRequest{UserId: "user-1"})
	require.Error(t, err)

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "finance.get_defaults", events[0].Tags["operation"])
}
