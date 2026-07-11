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
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func setupGRPCHandler(repo *mockFinanceRepository) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, new(mockTxBeginner), nil, time.Now, logger)
	return NewGRPCHandler(financeSvc, logger)
}

// TestGetDefaults_WrappedTypedErrorClassifies locks in C7: a typed *apierr.Error
// that has been %w-wrapped before reaching the gRPC handler must still classify
// via errors.As (not collapse to codes.Internal). The service wraps every repo
// error with %w ("getting defaults: %w"), so a typed NOT_FOUND returned by the
// repo reaches the handler wrapped; the handler must still map it to NotFound.
func TestGetDefaults_WrappedTypedErrorClassifies(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	repo.On("GetDefaults", mock.Anything, "user-1").
		Return(nil, apierr.NotFound("defaults missing"))

	resp, err := handler.GetDefaults(context.Background(), &pb.GetDefaultsRequest{UserId: "user-1"})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGetAllUserData_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	createdAt := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	tags := []*model.Tag{
		{ID: "tag-1", UserID: "user-1", Name: "Bills", IsDefault: true, CreatedAt: createdAt},
		{ID: "tag-2", UserID: "user-1", Name: "Food", IsDefault: true, CreatedAt: createdAt},
	}
	periods := []*model.BudgetPeriod{
		{
			ID: "period-1", UserID: "user-1", Year: 2026, Month: 3,
			BudgetAmount: 500000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
			CreatedAt: createdAt,
		},
	}
	defaults := &model.DefaultSettings{
		UserID:            "user-1",
		BudgetAmount:      500000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "GBP",
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}

	repo.On("ListTags", mock.Anything, "user-1").Return(tags, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").Return(periods, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(defaults, nil)

	resp, err := handler.GetAllUserData(context.Background(), &pb.GetAllUserDataRequest{UserId: "user-1"})
	require.NoError(t, err)

	// Tags
	require.Len(t, resp.Tags, 2)
	assert.Equal(t, "tag-1", resp.Tags[0].Id)
	assert.Equal(t, "Bills", resp.Tags[0].Name)
	assert.True(t, resp.Tags[0].IsDefault)
	assert.Equal(t, "2026-03-15T10:30:00Z", resp.Tags[0].CreatedAt)

	// Periods
	require.Len(t, resp.Periods, 1)
	assert.Equal(t, "period-1", resp.Periods[0].Id)
	assert.Equal(t, int32(2026), resp.Periods[0].Year)
	assert.Equal(t, int32(3), resp.Periods[0].Month)
	assert.Equal(t, int64(500000), resp.Periods[0].BudgetAmount)
	assert.Equal(t, "2026-03-15T10:30:00Z", resp.Periods[0].CreatedAt)

	// Defaults
	require.NotNil(t, resp.Defaults)
	assert.Equal(t, "user-1", resp.Defaults.UserId)
	assert.Equal(t, int64(500000), resp.Defaults.BudgetAmount)
	assert.Equal(t, "GBP", resp.Defaults.Currency)
}

func TestGetAllUserData_EmptyUser(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	repo.On("ListTags", mock.Anything, "user-empty").Return([]*model.Tag{}, nil)
	repo.On("ListPeriods", mock.Anything, "user-empty").Return([]*model.BudgetPeriod{}, nil)
	repo.On("GetDefaults", mock.Anything, "user-empty").Return((*model.DefaultSettings)(nil), nil)

	resp, err := handler.GetAllUserData(context.Background(), &pb.GetAllUserDataRequest{UserId: "user-empty"})
	require.NoError(t, err)

	assert.Empty(t, resp.Tags)
	assert.Empty(t, resp.Periods)
	assert.Nil(t, resp.Defaults)
}

func TestGetAllUserData_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	resp, err := handler.GetAllUserData(context.Background(), &pb.GetAllUserDataRequest{UserId: ""})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id is required")
}

func TestGetAllUserData_InternalError(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	// GetAllUserData fans out the three reads concurrently (no serial
	// short-circuit), so the sibling reads must be stubbed even though ListTags
	// is the one failing. The errgroup surfaces the ListTags error.
	repo.On("ListTags", mock.Anything, "user-1").Return(nil, fmt.Errorf("connection refused"))
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{}, nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return((*model.DefaultSettings)(nil), nil)

	resp, err := handler.GetAllUserData(context.Background(), &pb.GetAllUserDataRequest{UserId: "user-1"})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

// --- DeleteAllUserData Handler Tests ---

func TestDeleteAllUserData_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)
	handler := NewGRPCHandler(financeSvc, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)
	txRepo.On("DeleteAllUserData", mock.Anything, "user-1").Return(nil)

	resp, err := handler.DeleteAllUserData(context.Background(), &pb.DeleteAllUserDataRequest{UserId: "user-1"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	tx.AssertCalled(t, "Commit", mock.Anything)
	txRepo.AssertCalled(t, "DeleteAllUserData", mock.Anything, "user-1")
}

func TestDeleteAllUserData_Idempotent(t *testing.T) {
	// Calling for a user with no data should still return success
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)
	handler := NewGRPCHandler(financeSvc, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)
	// 0 rows deleted is still nil error (idempotent)
	txRepo.On("DeleteAllUserData", mock.Anything, "user-no-data").Return(nil)

	resp, err := handler.DeleteAllUserData(context.Background(), &pb.DeleteAllUserDataRequest{UserId: "user-no-data"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	tx.AssertCalled(t, "Commit", mock.Anything)
}

func TestDeleteAllUserData_EmptyUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	handler := setupGRPCHandler(repo)

	resp, err := handler.DeleteAllUserData(context.Background(), &pb.DeleteAllUserDataRequest{UserId: ""})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id is required")
}

func TestDeleteAllUserData_DatabaseError(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)
	handler := NewGRPCHandler(financeSvc, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	txRepo.On("DeleteAllUserData", mock.Anything, "user-1").Return(fmt.Errorf("connection refused"))

	resp, err := handler.DeleteAllUserData(context.Background(), &pb.DeleteAllUserDataRequest{UserId: "user-1"})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to delete user data")

	// Verify rollback was called (commit should NOT have been called)
	tx.AssertCalled(t, "Rollback", mock.Anything)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
}

func TestDeleteAllUserData_TransactionBeginError(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)
	handler := NewGRPCHandler(financeSvc, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(nil, fmt.Errorf("pool exhausted"))

	resp, err := handler.DeleteAllUserData(context.Background(), &pb.DeleteAllUserDataRequest{UserId: "user-1"})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}
