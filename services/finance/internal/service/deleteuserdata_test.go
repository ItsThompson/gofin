package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/repository"
)

// mockTxForDeletion implements repository.Tx for deletion tests.
type mockTxForDeletion struct {
	mock.Mock
	repo repository.FinanceRepository
}

func (m *mockTxForDeletion) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTxForDeletion) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTxForDeletion) Repo() repository.FinanceRepository {
	return m.repo
}

// mockTxBeginnerForDeletion implements repository.TxBeginner.
type mockTxBeginnerForDeletion struct {
	mock.Mock
}

func (m *mockTxBeginnerForDeletion) BeginTx(ctx context.Context) (repository.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.Tx), args.Error(1)
}

func TestDeleteAllUserData_ServiceSuccess(t *testing.T) {
	txRepo := new(mockRepo)
	txBeginner := new(mockTxBeginnerForDeletion)
	tx := &mockTxForDeletion{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(nil, txBeginner, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)
	txRepo.On("DeleteAllUserData", mock.Anything, "user-123").Return(nil)

	err := svc.DeleteAllUserData(context.Background(), "user-123")
	require.NoError(t, err)

	tx.AssertCalled(t, "Commit", mock.Anything)
	txRepo.AssertCalled(t, "DeleteAllUserData", mock.Anything, "user-123")
}

func TestDeleteAllUserData_ServiceIdempotent_NoData(t *testing.T) {
	txRepo := new(mockRepo)
	txBeginner := new(mockTxBeginnerForDeletion)
	tx := &mockTxForDeletion{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(nil, txBeginner, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)
	// User has no data: 0 rows deleted is still nil error
	txRepo.On("DeleteAllUserData", mock.Anything, "user-empty").Return(nil)

	err := svc.DeleteAllUserData(context.Background(), "user-empty")
	require.NoError(t, err)

	tx.AssertCalled(t, "Commit", mock.Anything)
}

func TestDeleteAllUserData_ServiceTransactionBeginError(t *testing.T) {
	txBeginner := new(mockTxBeginnerForDeletion)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(nil, txBeginner, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(nil, fmt.Errorf("pool exhausted"))

	err := svc.DeleteAllUserData(context.Background(), "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beginning transaction")
}

func TestDeleteAllUserData_ServiceRepositoryError_RollsBack(t *testing.T) {
	txRepo := new(mockRepo)
	txBeginner := new(mockTxBeginnerForDeletion)
	tx := &mockTxForDeletion{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(nil, txBeginner, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	txRepo.On("DeleteAllUserData", mock.Anything, "user-123").Return(fmt.Errorf("connection refused"))

	err := svc.DeleteAllUserData(context.Background(), "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting all user data")

	// Rollback should be called, commit should NOT
	tx.AssertCalled(t, "Rollback", mock.Anything)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
}

func TestDeleteAllUserData_ServiceCommitError(t *testing.T) {
	txRepo := new(mockRepo)
	txBeginner := new(mockTxBeginnerForDeletion)
	tx := &mockTxForDeletion{repo: txRepo}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewFinanceService(nil, txBeginner, logger)

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(fmt.Errorf("commit failed"))
	txRepo.On("DeleteAllUserData", mock.Anything, "user-123").Return(nil)

	err := svc.DeleteAllUserData(context.Background(), "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "committing transaction")
}
