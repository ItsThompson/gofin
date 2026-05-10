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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
)

// --- Mock: DeletionJobRepository ---

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

var _ repository.DeletionJobRepository = (*mockDeletionJobRepository)(nil)

// --- Mock: Export JobRepository ---

type mockExportJobRepository struct {
	mock.Mock
}

func (m *mockExportJobRepository) CreateJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockExportJobRepository) GetJob(ctx context.Context, jobID string) (*model.ExportJob, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockExportJobRepository) ListJobsByUser(ctx context.Context, userID string, page, pageSize int) ([]*model.ExportJob, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*model.ExportJob), args.Get(1).(int64), args.Error(2)
}

func (m *mockExportJobRepository) GetInProgressJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockExportJobRepository) GetLatestNonFailedJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ExportJob), args.Error(1)
}

func (m *mockExportJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
	args := m.Called(ctx, jobID, status)
	return args.Error(0)
}

func (m *mockExportJobRepository) CompleteJob(ctx context.Context, jobID string, fileSizeBytes int64) error {
	args := m.Called(ctx, jobID, fileSizeBytes)
	return args.Error(0)
}

func (m *mockExportJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	args := m.Called(ctx, jobID, errMsg)
	return args.Error(0)
}

func (m *mockExportJobRepository) GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.RecoverableJob), args.Error(1)
}

var _ repository.JobRepository = (*mockExportJobRepository)(nil)

// --- Mock: AuthServiceClient ---

type mockAuthServiceClient struct {
	verifyPasswordResp *authpb.VerifyPasswordResponse
	verifyPasswordErr  error
	getUserResp        *authpb.UserResponse
	getUserErr         error
}

func (m *mockAuthServiceClient) VerifyPassword(_ context.Context, _ *authpb.VerifyPasswordRequest, _ ...grpc.CallOption) (*authpb.VerifyPasswordResponse, error) {
	return m.verifyPasswordResp, m.verifyPasswordErr
}
func (m *mockAuthServiceClient) GetUser(_ context.Context, _ *authpb.GetUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return m.getUserResp, m.getUserErr
}
func (m *mockAuthServiceClient) Register(_ context.Context, _ *authpb.RegisterRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Login(_ context.Context, _ *authpb.LoginRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ValidateToken(_ context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) DeleteUserData(_ context.Context, _ *authpb.DeleteUserDataRequest, _ ...grpc.CallOption) (*authpb.DeleteUserDataResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) RefreshToken(_ context.Context, _ *authpb.RefreshTokenRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Logout(_ context.Context, _ *authpb.LogoutRequest, _ ...grpc.CallOption) (*authpb.LogoutResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) AssumeIdentity(_ context.Context, _ *authpb.AssumeIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) RestoreIdentity(_ context.Context, _ *authpb.RestoreIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ListUsers(_ context.Context, _ *authpb.ListUsersRequest, _ ...grpc.CallOption) (*authpb.ListUsersResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) UpdateUser(_ context.Context, _ *authpb.UpdateUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ChangePassword(_ context.Context, _ *authpb.ChangePasswordRequest, _ ...grpc.CallOption) (*authpb.ChangePasswordResponse, error) {
	return nil, nil
}

var _ authpb.AuthServiceClient = (*mockAuthServiceClient)(nil)

// --- Test helpers ---

func setupDeletionTestRouter(
	repo repository.DeletionJobRepository,
	authClient authpb.AuthServiceClient,
	exportRepo repository.JobRepository,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	opts := []service.DeletionServiceOption{}
	if authClient != nil {
		opts = append(opts, service.WithAuthClient(authClient))
	}
	if exportRepo != nil {
		opts = append(opts, service.WithExportRepo(exportRepo))
	}

	svc := service.NewDeletionService(repo, logger, opts...)
	h := NewDeletionHandler(svc, logger)

	router := gin.New()
	h.RegisterRoutes(router)
	return router
}

func validAuthClient() *mockAuthServiceClient {
	return &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserResp:        &authpb.UserResponse{Id: "target-user-id", Username: "alice"},
	}
}

// --- Handler Tests: POST /api/datarights/deletions ---

func TestCreateDeletion_NewJob_Returns202(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := validAuthClient()
	now := time.Now().UTC().Truncate(time.Millisecond)

	expectedJob := &model.DeletionJob{
		ID:          "del-job-001",
		UserID:      "target-user-id",
		AdminUserID: "admin-user-id",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user-id", "admin-user-id").Return(expectedJob, nil)

	router := setupDeletionTestRouter(mockRepo, authClient, mockExportRepo)

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
	mockExportRepo.AssertExpectations(t)
}

func TestCreateDeletion_ExistingJob_Returns200(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := validAuthClient()
	now := time.Now().UTC().Truncate(time.Millisecond)

	existingJob := &model.DeletionJob{
		ID:          "existing-job-001",
		UserID:      "target-user-id",
		AdminUserID: "other-admin",
		Status:      model.StatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(existingJob, nil)

	router := setupDeletionTestRouter(mockRepo, authClient, mockExportRepo)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.DeletionJobResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "existing-job-001", resp.ID)
	assert.Equal(t, model.StatusRunning, resp.Status)
	mockRepo.AssertExpectations(t)
	mockExportRepo.AssertExpectations(t)
}

func TestCreateDeletion_InvalidPassword_Returns401(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: false},
	}

	router := setupDeletionTestRouter(mockRepo, authClient, nil)

	body := `{"userId": "target-user-id", "password": "wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrInvalidCredentials, resp.Code)
}

func TestCreateDeletion_SelfDeletion_Returns400(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
	}

	router := setupDeletionTestRouter(mockRepo, authClient, nil)

	body := `{"userId": "admin-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Cannot delete your own account")
}

func TestCreateDeletion_ProtectedUsername_Returns403(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserResp:        &authpb.UserResponse{Id: "target-user-id", Username: "admin"},
	}

	router := setupDeletionTestRouter(mockRepo, authClient, nil)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrProtectedUser, resp.Code)
}

func TestCreateDeletion_ExportConflict_Returns409(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := validAuthClient()

	exportJob := &model.ExportJob{
		ID:     "export-001",
		UserID: "target-user-id",
		Status: model.StatusRunning,
	}
	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(exportJob, nil)

	router := setupDeletionTestRouter(mockRepo, authClient, mockExportRepo)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrExportConflict, resp.Code)
	assert.Contains(t, resp.Message, "Cannot delete user while data export is in progress")
	mockExportRepo.AssertExpectations(t)
}

func TestCreateDeletion_AuthServiceUnavailable_Returns503(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordErr: status.Error(codes.Unavailable, "connection refused"),
	}

	router := setupDeletionTestRouter(mockRepo, authClient, nil)

	body := `{"userId": "target-user-id", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/datarights/deletions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin-user-id")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrServiceUnavailable, resp.Code)
}

func TestCreateDeletion_Unauthenticated_Returns401(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	router := setupDeletionTestRouter(mockRepo, nil, nil)

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
	router := setupDeletionTestRouter(mockRepo, nil, nil)

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
	router := setupDeletionTestRouter(mockRepo, nil, nil)

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
	mockExportRepo := new(mockExportJobRepository)
	authClient := validAuthClient()

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user-id").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user-id", "admin-user-id").
		Return(nil, fmt.Errorf("connection refused"))

	router := setupDeletionTestRouter(mockRepo, authClient, mockExportRepo)

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
	mockExportRepo.AssertExpectations(t)
}

// --- Handler Tests: GET /api/datarights/deletions/:id ---

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

	router := setupDeletionTestRouter(mockRepo, nil, nil)
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

	router := setupDeletionTestRouter(mockRepo, nil, nil)
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
	router := setupDeletionTestRouter(mockRepo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/datarights/deletions/del-job-002", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp model.ApiError
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, model.ErrUnauthorized, resp.Code)
}

func TestGetDeletion_CompletedJob_ReturnsCompletedAt(t *testing.T) {
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

	router := setupDeletionTestRouter(mockRepo, nil, nil)
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
