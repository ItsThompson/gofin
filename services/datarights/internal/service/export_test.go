package service

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

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

type mockJobRepository struct {
	mock.Mock
}

func (m *mockJobRepository) CreateJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockJobRepository) GetJob(ctx context.Context, jobID string) (*model.ExportJob, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockJobRepository) ListJobsByUser(ctx context.Context, userID string, page, pageSize int) ([]*model.ExportJob, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*model.ExportJob), args.Get(1).(int64), args.Error(2)
}

var _ repository.JobRepository = (*mockJobRepository)(nil)

func newTestService(repo repository.JobRepository) *ExportService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewExportService(repo, logger)
}

func TestCreateJob_Success(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	expectedJob := &model.ExportJob{
		ID:        "job-id-1",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(expectedJob, nil)

	svc := newTestService(mockRepo)
	job, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "job-id-1", job.ID)
	assert.Equal(t, "user-123", job.UserID)
	assert.Equal(t, model.StatusPending, job.Status)
	mockRepo.AssertExpectations(t)
}

func TestCreateJob_RepoError(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(nil, fmt.Errorf("db connection failed"))

	svc := newTestService(mockRepo)
	job, err := svc.CreateJob(context.Background(), "user-123")

	assert.Nil(t, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating job")
}

func TestGetJob_Success(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	expectedJob := &model.ExportJob{
		ID:        "job-id-1",
		UserID:    "user-123",
		Status:    model.StatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mockRepo.On("GetJob", mock.Anything, "job-id-1").Return(expectedJob, nil)

	svc := newTestService(mockRepo)
	job, err := svc.GetJob(context.Background(), "job-id-1", "user-123")

	require.NoError(t, err)
	assert.Equal(t, "job-id-1", job.ID)
	assert.Equal(t, model.StatusRunning, job.Status)
}

func TestGetJob_NotFound(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetJob", mock.Anything, "nonexistent").Return(nil, nil)

	svc := newTestService(mockRepo)
	job, err := svc.GetJob(context.Background(), "nonexistent", "user-123")

	assert.Nil(t, job)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, 404, svcErr.Status)
	assert.Equal(t, model.ErrNotFound, svcErr.Code)
}

func TestGetJob_DifferentUser(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	job := &model.ExportJob{
		ID:        "job-id-1",
		UserID:    "other-user",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mockRepo.On("GetJob", mock.Anything, "job-id-1").Return(job, nil)

	svc := newTestService(mockRepo)
	result, err := svc.GetJob(context.Background(), "job-id-1", "user-123")

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr, ok := err.(*ServiceError)
	require.True(t, ok)
	assert.Equal(t, 404, svcErr.Status)
}

func TestListJobs_Success(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	jobs := []*model.ExportJob{
		{ID: "j1", UserID: "user-123", Status: model.StatusCompleted, CreatedAt: now, UpdatedAt: now},
		{ID: "j2", UserID: "user-123", Status: model.StatusFailed, CreatedAt: now, UpdatedAt: now},
	}
	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 10).Return(jobs, int64(2), nil)

	svc := newTestService(mockRepo)
	result, err := svc.ListJobs(context.Background(), "user-123", 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, int64(2), result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
	assert.False(t, result.HasMore)
}

func TestListJobs_EnforcesMaxPageSize(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 50).Return([]*model.ExportJob{}, int64(0), nil)

	svc := newTestService(mockRepo)
	result, err := svc.ListJobs(context.Background(), "user-123", 1, 100)

	require.NoError(t, err)
	assert.Equal(t, 50, result.PageSize)
	mockRepo.AssertCalled(t, "ListJobsByUser", mock.Anything, "user-123", 1, 50)
}

func TestListJobs_DefaultsInvalidPage(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 10).Return([]*model.ExportJob{}, int64(0), nil)

	svc := newTestService(mockRepo)
	result, err := svc.ListJobs(context.Background(), "user-123", -1, -5)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
}

func TestListJobs_EmptyResult(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 10).Return(nil, int64(0), nil)

	svc := newTestService(mockRepo)
	result, err := svc.ListJobs(context.Background(), "user-123", 1, 10)

	require.NoError(t, err)
	assert.NotNil(t, result.Data)
	assert.Equal(t, 0, len(result.Data))
}
