package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
)

// mockJobRepository implements repository.JobRepository for handler tests.
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

// Ensure mockJobRepository satisfies the interface.
var _ repository.JobRepository = (*mockJobRepository)(nil)

func setupTestRouter(repo repository.JobRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewExportService(repo, logger)
	rest := NewRESTHandler(svc, logger)
	deletion := NewDeletionHandler(nil, logger)

	router := gin.New()
	RegisterRoutes(router, rest, deletion)
	return router
}

func TestCreateExport_NewJob_Returns202(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	expectedJob := &model.ExportJob{
		ID:        "test-job-id",
		UserID:    "user-123",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "user-123").Return(expectedJob, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var resp model.JobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-job-id", resp.Job.ID)
	assert.Equal(t, "user-123", resp.Job.UserID)
	assert.Equal(t, model.StatusPending, resp.Job.Status)
}

func TestCreateExport_InProgressDedup_Returns200(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	existingJob := &model.ExportJob{
		ID:        "existing-job",
		UserID:    "user-123",
		Status:    model.StatusRunning,
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(existingJob, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.JobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "existing-job", resp.Job.ID)
	assert.Equal(t, model.StatusRunning, resp.Job.Status)
}

func TestCreateExport_RateLimited_Returns429(t *testing.T) {
	mockRepo := new(mockJobRepository)
	fiveDaysAgo := time.Now().UTC().Add(-5 * 24 * time.Hour).Truncate(time.Millisecond)

	latestJob := &model.ExportJob{
		ID:        "recent-job",
		UserID:    "user-123",
		Status:    model.StatusCompleted,
		CreatedAt: fiveDaysAgo,
		UpdatedAt: fiveDaysAgo,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(latestJob, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	var resp model.RateLimitedResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrRateLimited, resp.Code)
	assert.Contains(t, resp.Message, "Export limit reached")

	expectedRetryAfter := fiveDaysAgo.Add(service.RateLimitWindow)
	assert.Equal(t, expectedRetryAfter.Unix(), resp.RetryAfter.Unix())
}

func TestCreateExport_RateLimited_IncludesRetryAfterTimestamp(t *testing.T) {
	mockRepo := new(mockJobRepository)
	twoDaysAgo := time.Now().UTC().Add(-2 * 24 * time.Hour).Truncate(time.Second)

	latestJob := &model.ExportJob{
		ID:        "recent-job",
		UserID:    "user-123",
		Status:    model.StatusCompleted,
		CreatedAt: twoDaysAgo,
		UpdatedAt: twoDaysAgo,
	}

	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, nil)
	mockRepo.On("GetLatestNonFailedJob", mock.Anything, "user-123").Return(latestJob, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	var resp model.RateLimitedResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	// retryAfter should be createdAt + 30 days
	expectedRetry := twoDaysAgo.Add(30 * 24 * time.Hour)
	assert.Equal(t, expectedRetry.Unix(), resp.RetryAfter.Unix())
	assert.False(t, resp.RetryAfter.IsZero())
}

func TestCreateExport_DBError_Returns500(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetInProgressJob", mock.Anything, "user-123").Return(nil, fmt.Errorf("connection refused"))

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp apierr.APIError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, apierr.CodeInternal, resp.Code)
}

func TestCreateExport_Unauthenticated(t *testing.T) {
	mockRepo := new(mockJobRepository)
	router := setupTestRouter(mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/datarights/exports", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp apierr.APIError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, apierr.CodeUnauthorized, resp.Code)
}

func TestGetExport_Success(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	job := &model.ExportJob{
		ID:        "job-456",
		UserID:    "user-123",
		Status:    model.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetJob", mock.Anything, "job-456").Return(job, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports/job-456", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.JobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "job-456", resp.Job.ID)
	assert.Equal(t, model.StatusCompleted, resp.Job.Status)
}

func TestGetExport_NotFound(t *testing.T) {
	mockRepo := new(mockJobRepository)
	mockRepo.On("GetJob", mock.Anything, "nonexistent").Return(nil, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports/nonexistent", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp apierr.APIError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, apierr.CodeNotFound, resp.Code)
}

func TestGetExport_DifferentUser(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	job := &model.ExportJob{
		ID:        "job-456",
		UserID:    "other-user",
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRepo.On("GetJob", mock.Anything, "job-456").Return(job, nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports/job-456", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListExports_Success(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	jobs := []*model.ExportJob{
		{ID: "job-1", UserID: "user-123", Status: model.StatusCompleted, CreatedAt: now, UpdatedAt: now},
		{ID: "job-2", UserID: "user-123", Status: model.StatusPending, CreatedAt: now, UpdatedAt: now},
	}

	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 10).Return(jobs, int64(2), nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.JobListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Data))
	assert.Equal(t, int64(2), resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PageSize)
	assert.False(t, resp.HasMore)
}

func TestListExports_Pagination(t *testing.T) {
	mockRepo := new(mockJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	jobs := []*model.ExportJob{
		{ID: "job-1", UserID: "user-123", Status: model.StatusCompleted, CreatedAt: now, UpdatedAt: now},
	}

	mockRepo.On("ListJobsByUser", mock.Anything, "user-123", 1, 1).Return(jobs, int64(3), nil)

	router := setupTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports?page=1&pageSize=1", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.JobListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, int64(3), resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 1, resp.PageSize)
	assert.True(t, resp.HasMore)
}

func TestListExports_Unauthenticated(t *testing.T) {
	mockRepo := new(mockJobRepository)
	router := setupTestRouter(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/datarights/exports", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
