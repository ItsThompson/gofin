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

	"github.com/ItsThompson/gofin/services/apierr"
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

func (m *mockJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
	args := m.Called(ctx, jobID, status)
	return args.Error(0)
}

func (m *mockJobRepository) CompleteJob(ctx context.Context, jobID string, fileSizeBytes int64) error {
	args := m.Called(ctx, jobID, fileSizeBytes)
	return args.Error(0)
}

func (m *mockJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	args := m.Called(ctx, jobID, errMsg)
	return args.Error(0)
}

func (m *mockJobRepository) GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.RecoverableJob), args.Error(1)
}

func (m *mockJobRepository) GetInProgressJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockJobRepository) GetLatestNonFailedJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

var _ repository.JobRepository = (*mockJobRepository)(nil)

func newTestService(repo repository.JobRepository) *ExportService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewExportService(repo, logger)
}

func TestCreateJob_NoPriorExports_Allowed(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	expectedJob := &model.ExportJob{
		ID:        "job-id-1",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(expectedJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "job-id-1", result.Job.ID)
	assert.Equal(t, "user-123", result.Job.UserID)
	assert.Equal(t, model.StatusPending, result.Job.Status)
	assert.False(t, result.IsExisting)
	mockRepo.AssertExpectations(t)
}

func TestCreateJob_WithinRateLimit_Denied(t *testing.T) {
	mockRepo := new(mockJobRepository)
	// Job created 10 days ago (within 30-day window)
	tenDaysAgo := time.Now().UTC().Add(-10 * 24 * time.Hour)

	latestJob := &model.ExportJob{
		ID:        "old-job",
		UserID:    "user-123",
		Status:    model.StatusCompleted,
		CreatedAt: tenDaysAgo,
		UpdatedAt: tenDaysAgo,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(latestJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	assert.Nil(t, result)
	require.Error(t, err)

	rateLimitErr, ok := err.(*RateLimitError)
	require.True(t, ok, "expected RateLimitError, got %T", err)

	expectedRetryAfter := tenDaysAgo.Add(RateLimitWindow)
	assert.Equal(t, expectedRetryAfter.Unix(), rateLimitErr.RetryAfter.Unix())
}

func TestCreateJob_Exactly30DaysAgo_Allowed(t *testing.T) {
	mockRepo := new(mockJobRepository)
	// Job created slightly more than 30 days ago (should be allowed)
	justOverThirtyDays := time.Now().UTC().Add(-RateLimitWindow).Add(-time.Second)

	latestJob := &model.ExportJob{
		ID:        "old-job",
		UserID:    "user-123",
		Status:    model.StatusCompleted,
		CreatedAt: justOverThirtyDays,
		UpdatedAt: justOverThirtyDays,
	}

	newJob := &model.ExportJob{
		ID:        "new-job",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(latestJob, nil)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(newJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "new-job", result.Job.ID)
	assert.False(t, result.IsExisting)
	mockRepo.AssertExpectations(t)
}

func TestCreateJob_LastJobFailed_Allowed(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	// GetLatestNonFailedJob returns nil because all previous jobs are failed
	newJob := &model.ExportJob{
		ID:        "new-job",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(newJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "new-job", result.Job.ID)
	assert.False(t, result.IsExisting)
	mockRepo.AssertExpectations(t)
}

func TestCreateJob_InProgressDedup_ReturnsExisting(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	existingJob := &model.ExportJob{
		ID:        "existing-job",
		UserID:    "user-123",
		Status:    model.StatusRunning,
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(existingJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "existing-job", result.Job.ID)
	assert.Equal(t, model.StatusRunning, result.Job.Status)
	assert.True(t, result.IsExisting)

	// Should not check rate limit or create job when dedup fires
	mockRepo.AssertNotCalled(t, "GetLatestNonFailedJob", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "CreateJob", mock.Anything, mock.Anything)
}

func TestCreateJob_InProgressPendingDedup_ReturnsExisting(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC()

	existingJob := &model.ExportJob{
		ID:        "pending-job",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: now.Add(-1 * time.Minute),
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(existingJob, nil)

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "pending-job", result.Job.ID)
	assert.Equal(t, model.StatusPending, result.Job.Status)
	assert.True(t, result.IsExisting)
}

func TestCreateJob_InProgressCheckError_FailsClosed(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, fmt.Errorf("db connection lost"))

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking in-progress job")

	// Must not create a job if we can't verify dedup status (fail closed)
	mockRepo.AssertNotCalled(t, "CreateJob", mock.Anything, mock.Anything)
}

func TestCreateJob_RateLimitCheckError_FailsClosed(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(nil, fmt.Errorf("timeout"))

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking rate limit")

	// Must not create a job if we can't verify rate limit (fail closed)
	mockRepo.AssertNotCalled(t, "CreateJob", mock.Anything, mock.Anything)
}

func TestCreateJob_CreateRepoError(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(nil, fmt.Errorf("db connection failed"))

	svc := newTestService(mockRepo)
	result, err := svc.CreateJob(context.Background(), "user-123")

	assert.Nil(t, result)
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

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, 404, svcErr.Status)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
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

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
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
