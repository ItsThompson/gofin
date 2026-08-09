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
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// repoFailure is the untyped repository error every internal-failure case
// injects. It stands in for any infrastructure fault the handler cannot
// classify, and it must never reach the gRPC status message.
const repoFailure = "connection refused"

// setupGRPCHandlerWithLog builds a gRPC handler whose records land in the returned
// buffer, so a test can assert on what the handler recorded.
//
// The sink is installed as slog.Default because errkit writes its record through
// the package-level logger, which every service main sets to its own.
func setupGRPCHandlerWithLog(t *testing.T, repo *mockFinanceRepository, txBeginner *mockTxBeginner) (*GRPCHandler, *bytes.Buffer) {
	t.Helper()

	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)
	return NewGRPCHandler(financeSvc, logger), buf
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

// TestGRPC_InternalFailure_LogsCauseAndKeepsItOffTheWire covers every bare
// codes.Internal exit in the gRPC handler: an unclassifiable repository error
// must produce codes.Internal, exactly one error-level record naming the cause,
// and a status message that still carries no internal detail.
func TestGRPC_InternalFailure_LogsCauseAndKeepsItOffTheWire(t *testing.T) {
	cases := []struct {
		name          string
		arrange       func(repo *mockFinanceRepository, txBeginner *mockTxBeginner)
		invoke        func(h *GRPCHandler) error
		wantStatusMsg string
		wantLogMsg    string
		wantAttrs     map[string]string
	}{
		{
			name: "GetDefaults",
			arrange: func(repo *mockFinanceRepository, _ *mockTxBeginner) {
				repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.GetDefaults(context.Background(), &pb.GetDefaultsRequest{UserId: "user-1"})
				return err
			},
			wantStatusMsg: "failed to get defaults",
			wantLogMsg:    "failed to get defaults",
			wantAttrs:     map[string]string{"method": "GetDefaults", "operation": "finance.get_defaults", "user_id": "user-1"},
		},
		{
			name: "CompleteOnboarding",
			arrange: func(_ *mockFinanceRepository, txBeginner *mockTxBeginner) {
				txBeginner.On("BeginTx", mock.Anything).Return(nil, errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.CompleteOnboarding(context.Background(), &pb.CompleteOnboardingRequest{
					UserId:            "user-1",
					BudgetAmount:      300000,
					EssentialsPercent: 50,
					DesiresPercent:    30,
					SavingsPercent:    20,
					Currency:          "USD",
				})
				return err
			},
			wantStatusMsg: "failed to complete onboarding",
			wantLogMsg:    "failed to complete onboarding",
			wantAttrs:     map[string]string{"method": "CompleteOnboarding", "operation": "finance.complete_onboarding", "user_id": "user-1"},
		},
		{
			name: "ListTags",
			arrange: func(repo *mockFinanceRepository, _ *mockTxBeginner) {
				repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(0), errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.ListTags(context.Background(), &pb.ListTagsRequest{UserId: "user-1"})
				return err
			},
			wantStatusMsg: "failed to list tags",
			wantLogMsg:    "failed to list tags",
			wantAttrs:     map[string]string{"method": "ListTags", "operation": "finance.list_tags", "user_id": "user-1"},
		},
		{
			name: "CreateTag",
			arrange: func(repo *mockFinanceRepository, _ *mockTxBeginner) {
				repo.On("CreateTag", mock.Anything, "user-1", "Groceries", false).
					Return(nil, errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.CreateTag(context.Background(), &pb.CreateTagRequest{UserId: "user-1", Name: "Groceries"})
				return err
			},
			wantStatusMsg: "failed to create tag",
			wantLogMsg:    "failed to create tag",
			wantAttrs:     map[string]string{"method": "CreateTag", "operation": "finance.create_tag", "user_id": "user-1"},
		},
		{
			name: "UpdateTag",
			arrange: func(repo *mockFinanceRepository, _ *mockTxBeginner) {
				repo.On("UpdateTag", mock.Anything, "tag-1", "user-1", "Groceries").
					Return(nil, errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.UpdateTag(context.Background(), &pb.UpdateTagRequest{
					UserId: "user-1", TagId: "tag-1", Name: "Groceries",
				})
				return err
			},
			wantStatusMsg: "failed to update tag",
			wantLogMsg:    "failed to update tag",
			wantAttrs:     map[string]string{"method": "UpdateTag", "operation": "finance.update_tag", "user_id": "user-1", "tag_id": "tag-1"},
		},
		{
			name: "DeleteTag",
			arrange: func(repo *mockFinanceRepository, _ *mockTxBeginner) {
				repo.On("GetTag", mock.Anything, "tag-1", "user-1").Return(nil, errors.New(repoFailure))
			},
			invoke: func(h *GRPCHandler) error {
				_, err := h.DeleteTag(context.Background(), &pb.DeleteTagRequest{UserId: "user-1", TagId: "tag-1"})
				return err
			},
			wantStatusMsg: "failed to delete tag",
			wantLogMsg:    "failed to delete tag",
			wantAttrs:     map[string]string{"method": "DeleteTag", "operation": "finance.delete_tag", "user_id": "user-1", "tag_id": "tag-1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockFinanceRepository)
			txBeginner := new(mockTxBeginner)
			handler, logs := setupGRPCHandlerWithLog(t, repo, txBeginner)
			tc.arrange(repo, txBeginner)

			err := tc.invoke(handler)
			require.Error(t, err)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.Internal, st.Code())
			assert.Equal(t, tc.wantStatusMsg, st.Message())
			assert.NotContains(t, st.Message(), repoFailure, "the cause must not reach the caller")

			records := errorRecords(t, logs)
			require.Len(t, records, 1)
			assert.Equal(t, tc.wantLogMsg, records[0]["msg"])
			assert.Contains(t, records[0]["error"], repoFailure)
			for attr, want := range tc.wantAttrs {
				assert.Equal(t, want, records[0][attr])
			}
		})
	}
}

// TestGRPC_ListTags_TypedInternalError_LogsCause covers ListTags' second
// codes.Internal exit, the one that forwards a typed *apierr.Error message to
// the caller. The wire message is deliberately unchanged, so the record is what
// carries the wrapped cause.
func TestGRPC_ListTags_TypedInternalError_LogsCause(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, logs := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("CountUserTags", mock.Anything, "user-1").
		Return(int64(0), apierr.Internal("Tag store unavailable"))

	_, err := handler.ListTags(context.Background(), &pb.ListTagsRequest{UserId: "user-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "Tag store unavailable", st.Message())

	records := errorRecords(t, logs)
	require.Len(t, records, 1)
	assert.Equal(t, "failed to list tags", records[0]["msg"])
	assert.Equal(t, "counting user tags: Tag store unavailable", records[0]["error"])
}

// TestGRPC_ClassifiedError_IsNotLoggedAsInternal asserts the handler only
// records the failures it cannot classify: a typed NOT_FOUND is an expected
// client outcome, so it maps to codes.NotFound and emits no error record.
func TestGRPC_ClassifiedError_IsNotLoggedAsInternal(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	handler, logs := setupGRPCHandlerWithLog(t, repo, txBeginner)

	repo.On("GetTag", mock.Anything, "tag-1", "user-1").Return((*model.Tag)(nil), nil)

	_, err := handler.DeleteTag(context.Background(), &pb.DeleteTagRequest{UserId: "user-1", TagId: "tag-1"})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Empty(t, errorRecords(t, logs))
}
