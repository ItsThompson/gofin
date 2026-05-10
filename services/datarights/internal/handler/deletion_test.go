package handler

import (
	"bytes"
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

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
)

// mockDeletionJobRepository implements repository.DeletionJobRepository for handler tests.
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

func setupDeletionTestRouter(repo repository.DeletionJobRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewDeletionService(repo, logger)
	h := NewDeletionHandler(svc, logger)

	router := gin.New()
	h.RegisterRoutes(router)
	return router
}

func TestCreateDeletion_Success_Returns202(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	expectedJob := &model.DeletionJob{
		ID:          "del-job-001",
		UserID:      "target-user-id",
		AdminUserID: "admin-user-id",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockRepo.On("CreateJob", mock.Anything, "target-user-id", "admin-user-id").Return(expectedJob, nil)

	router := setupDeletionTestRouter(mockRepo)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var resp model.DeletionJobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "del-job-001", resp.ID)
	assert.Equal(t, "target-user-id", resp.UserID)
	assert.Equal(t, model.StatusPending, resp.Status)
	assert.Nil(t, resp.Error)
	mockRepo.AssertExpectations(t)
}

func TestCreateDeletion_Unauthenticated_Returns401(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	router := setupDeletionTestRouter(mockRepo)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrUnauthorized, resp.Code)
}

func TestCreateDeletion_MissingUserId_Returns400(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	router := setupDeletionTestRouter(mockRepo)

	body := `{"password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrValidationError, resp.Code)
}

func TestCreateDeletion_MissingPassword_Returns400(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	router := setupDeletionTestRouter(mockRepo)

	body := `{"userId": "target-user-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrValidationError, resp.Code)
}

func TestCreateDeletion_DBError_Returns500(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockRepo.On("CreateJob", mock.Anything, "target-user-id", "admin-user-id").
		Return(nil, fmt.Errorf("connection refused"))

	router := setupDeletionTestRouter(mockRepo)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrInternalServerError, resp.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetDeletion_Success_Returns200(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	job := &model.DeletionJob{
		ID:          "del-job-002",
		UserID:      "target-user-id",
		AdminUserID: "admin-user-id",
		Status:      model.StatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockRepo.On("GetJob", mock.Anything, "del-job-002").Return(job, nil)

	router := setupDeletionTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/deletions/del-job-002", nil)
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.DeletionJobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "del-job-002", resp.ID)
	assert.Equal(t, "target-user-id", resp.UserID)
	assert.Equal(t, model.StatusRunning, resp.Status)
	mockRepo.AssertExpectations(t)
}

func TestGetDeletion_NotFound_Returns404(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockRepo.On("GetJob", mock.Anything, "nonexistent-id").Return(nil, nil)

	router := setupDeletionTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/deletions/nonexistent-id", nil)
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrNotFound, resp.Code)
	mockRepo.AssertExpectations(t)
}

func TestGetDeletion_Unauthenticated_Returns401(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	router := setupDeletionTestRouter(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/datarights/deletions/del-job-002", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrUnauthorized, resp.Code)
}

func TestGetDeletion_AdminUserIDFromHeader(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	now := time.Now().UTC().Truncate(time.Millisecond)

	job := &model.DeletionJob{
		ID:          "del-job-003",
		UserID:      "target-user-id",
		AdminUserID: "admin-user-id",
		Status:      model.StatusCompleted,
		CreatedAt:   now,
		CompletedAt: &now,
		UpdatedAt:   now,
	}

	mockRepo.On("GetJob", mock.Anything, "del-job-003").Return(job, nil)

	router := setupDeletionTestRouter(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/datarights/deletions/del-job-003", nil)
	req.Header.Set("X-User-ID", "some-admin")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.DeletionJobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCompleted, resp.Status)
	assert.NotNil(t, resp.CompletedAt)
	mockRepo.AssertExpectations(t)
}
