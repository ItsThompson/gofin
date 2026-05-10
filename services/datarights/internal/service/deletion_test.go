package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// mockDeletionJobRepository implements repository.DeletionJobRepository for tests.
type mockDeletionJobRepository struct {
	mock.Mock
}

func (m *mockDeletionJobRepository) CreateJob(ctx context.Context, userID, adminUserID string) (*model.DeletionJob, error) {
	args := m.Called(ctx, userID, adminUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DeletionJob), args.Error(1)
}

func (m *mockDeletionJobRepository) GetJob(ctx context.Context, jobID string) (*model.DeletionJob, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DeletionJob), args.Error(1)
}

func (m *mockDeletionJobRepository) GetInProgressJob(ctx context.Context, userID string) (*model.DeletionJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DeletionJob), args.Error(1)
}

func (m *mockDeletionJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
	args := m.Called(ctx, jobID, status)
	return args.Error(0)
}

func (m *mockDeletionJobRepository) CompleteJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *mockDeletionJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	args := m.Called(ctx, jobID, errMsg)
	return args.Error(0)
}

func (m *mockDeletionJobRepository) GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableDeletionJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.RecoverableDeletionJob), args.Error(1)
}

// Ensure mockDeletionJobRepository satisfies the interface.
var _ repository.DeletionJobRepository = (*mockDeletionJobRepository)(nil)

func newTestDeletionService(repo repository.DeletionJobRepository) *DeletionService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewDeletionService(repo, logger)
}

func TestDeletionService_CreateJob_Success(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo)

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-123",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	job, err := svc.CreateJob(context.Background(), "target-user", "admin-user")

	require.NoError(t, err)
	assert.Equal(t, "del-job-123", job.ID)
	assert.Equal(t, "target-user", job.UserID)
	assert.Equal(t, "admin-user", job.AdminUserID)
	assert.Equal(t, model.StatusPending, job.Status)
	mockRepo.AssertExpectations(t)
}

func TestDeletionService_CreateJob_DBError(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo)

	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").
		Return(nil, fmt.Errorf("connection refused"))

	job, err := svc.CreateJob(context.Background(), "target-user", "admin-user")

	assert.Nil(t, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	mockRepo.AssertExpectations(t)
}

func TestDeletionService_GetJob_Success(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo)

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-456",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockRepo.On("GetJob", mock.Anything, "del-job-456").Return(expectedJob, nil)

	job, err := svc.GetJob(context.Background(), "del-job-456")

	require.NoError(t, err)
	assert.Equal(t, "del-job-456", job.ID)
	assert.Equal(t, model.StatusRunning, job.Status)
	mockRepo.AssertExpectations(t)
}

func TestDeletionService_GetJob_NotFound(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo)

	mockRepo.On("GetJob", mock.Anything, "nonexistent").Return(nil, nil)

	job, err := svc.GetJob(context.Background(), "nonexistent")

	assert.Nil(t, job)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
	assert.Equal(t, http.StatusNotFound, svcErr.Status)
	mockRepo.AssertExpectations(t)
}

func TestDeletionService_GetJob_DBError(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo)

	mockRepo.On("GetJob", mock.Anything, "del-job-789").
		Return(nil, fmt.Errorf("timeout"))

	job, err := svc.GetJob(context.Background(), "del-job-789")

	assert.Nil(t, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	mockRepo.AssertExpectations(t)
}
