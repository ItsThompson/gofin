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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
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

// --- Mock: JobRepository (export) ---

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

func (m *mockAuthServiceClient) VerifyPassword(_ context.Context, req *authpb.VerifyPasswordRequest, _ ...grpc.CallOption) (*authpb.VerifyPasswordResponse, error) {
	return m.verifyPasswordResp, m.verifyPasswordErr
}

func (m *mockAuthServiceClient) GetUser(_ context.Context, req *authpb.GetUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
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

// --- Mock: DeletionJobSubmitter ---

type mockDeletionJobSubmitter struct {
	submitted []struct {
		jobID  string
		userID string
	}
}

func (m *mockDeletionJobSubmitter) Submit(jobID, userID string) {
	m.submitted = append(m.submitted, struct {
		jobID  string
		userID string
	}{jobID, userID})
}

var _ DeletionJobSubmitter = (*mockDeletionJobSubmitter)(nil)

// --- Test helpers ---

func newTestDeletionService(
	repo repository.DeletionJobRepository,
	authClient authpb.AuthServiceClient,
	exportRepo repository.JobRepository,
	engine DeletionJobSubmitter,
) *DeletionService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	opts := []DeletionServiceOption{}
	if authClient != nil {
		opts = append(opts, WithAuthClient(authClient))
	}
	if exportRepo != nil {
		opts = append(opts, WithExportRepo(exportRepo))
	}
	if engine != nil {
		opts = append(opts, WithDeletionEngine(engine))
	}
	return NewDeletionService(repo, logger, opts...)
}

func buildValidAuthClient() *mockAuthServiceClient {
	return &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserResp: &authpb.UserResponse{
			Id:       "target-user",
			Username: "alice",
		},
	}
}

// --- Tests: Guard 1 - Password Verification ---

func TestDeletionService_CreateJob_InvalidPassword_Returns401(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: false},
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "wrong-password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrInvalidCredentials, svcErr.Code)
	assert.Equal(t, http.StatusUnauthorized, svcErr.Status)
}

func TestDeletionService_CreateJob_ValidPassword_Passes(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()
	mockEngine := &mockDeletionJobSubmitter{}

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-123",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, mockEngine)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "correct-password")

	require.NoError(t, err)
	assert.Equal(t, "del-job-123", result.Job.ID)
	assert.False(t, result.IsExisting)
	mockRepo.AssertExpectations(t)
	mockExportRepo.AssertExpectations(t)
}

func TestDeletionService_CreateJob_AuthServiceUnavailable_Returns503(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordErr: status.Error(codes.Unavailable, "service unavailable"),
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrServiceUnavailable, svcErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, svcErr.Status)
}

// --- Tests: Guard 2 - Self-Deletion Prevention ---

func TestDeletionService_CreateJob_SelfDeletion_Returns400(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "admin-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, http.StatusBadRequest, svcErr.Status)
	assert.Contains(t, svcErr.Message, "Cannot delete your own account")
}

// --- Tests: Guard 3 - Protected Username Enforcement ---

func TestDeletionService_CreateJob_ProtectedUsername_Admin_Returns403(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserResp:        &authpb.UserResponse{Id: "target-user", Username: "admin"},
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrProtectedUser, svcErr.Code)
	assert.Equal(t, http.StatusForbidden, svcErr.Status)
}

func TestDeletionService_CreateJob_ProtectedUsername_Thompson_Returns403(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserResp:        &authpb.UserResponse{Id: "target-user", Username: "thompson"},
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrProtectedUser, svcErr.Code)
	assert.Equal(t, http.StatusForbidden, svcErr.Status)
}

func TestDeletionService_CreateJob_NonProtectedUsername_Passes(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()
	mockEngine := &mockDeletionJobSubmitter{}

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-123",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, mockEngine)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	require.NoError(t, err)
	assert.NotNil(t, result.Job)
	assert.False(t, result.IsExisting)
}

func TestDeletionService_CreateJob_GetUser_Unavailable_Returns503(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	authClient := &mockAuthServiceClient{
		verifyPasswordResp: &authpb.VerifyPasswordResponse{Valid: true},
		getUserErr:         status.Error(codes.Unavailable, "service unavailable"),
	}

	svc := newTestDeletionService(mockRepo, authClient, nil, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrServiceUnavailable, svcErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, svcErr.Status)
}

// --- Tests: Guard 4 - Export Conflict Detection ---

func TestDeletionService_CreateJob_ExportInProgress_Returns409(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()

	exportJob := &model.ExportJob{
		ID:     "export-job-001",
		UserID: "target-user",
		Status: model.StatusRunning,
	}
	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(exportJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrExportConflict, svcErr.Code)
	assert.Equal(t, http.StatusConflict, svcErr.Status)
	assert.Contains(t, svcErr.Message, "Cannot delete user while data export is in progress")
	mockExportRepo.AssertExpectations(t)
}

func TestDeletionService_CreateJob_NoExportConflict_Passes(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()
	mockEngine := &mockDeletionJobSubmitter{}

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-123",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, mockEngine)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	require.NoError(t, err)
	assert.NotNil(t, result.Job)
}

// --- Tests: Guard 5 - Idempotent Dedup ---

func TestDeletionService_CreateJob_ExistingJob_Returns200WithExisting(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()

	now := time.Now().UTC().Truncate(time.Millisecond)
	existingJob := &model.DeletionJob{
		ID:          "existing-job-001",
		UserID:      "target-user",
		AdminUserID: "other-admin",
		Status:      model.StatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(existingJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	require.NoError(t, err)
	assert.True(t, result.IsExisting)
	assert.Equal(t, "existing-job-001", result.Job.ID)
	assert.Equal(t, model.StatusRunning, result.Job.Status)
	mockRepo.AssertExpectations(t)
	mockExportRepo.AssertExpectations(t)
}

// --- Tests: Happy Path - Job Creation + Engine Submit ---

func TestDeletionService_CreateJob_AllGuardsPass_CreatesJobAndSubmits(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()
	mockEngine := &mockDeletionJobSubmitter{}

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-new",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, mockEngine)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	require.NoError(t, err)
	assert.False(t, result.IsExisting)
	assert.Equal(t, "del-job-new", result.Job.ID)
	assert.Equal(t, model.StatusPending, result.Job.Status)

	// Verify engine was called
	require.Len(t, mockEngine.submitted, 1)
	assert.Equal(t, "del-job-new", mockEngine.submitted[0].jobID)
	assert.Equal(t, "target-user", mockEngine.submitted[0].userID)

	mockRepo.AssertExpectations(t)
	mockExportRepo.AssertExpectations(t)
}

func TestDeletionService_CreateJob_DBError_ReturnsError(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(nil, fmt.Errorf("connection refused"))

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, nil)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "password")

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	mockRepo.AssertExpectations(t)
}

// --- Tests: GetJob ---

func TestDeletionService_GetJob_Success(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo, nil, nil, nil)

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
	svc := newTestDeletionService(mockRepo, nil, nil, nil)

	mockRepo.On("GetJob", mock.Anything, "nonexistent").Return(nil, nil)

	job, err := svc.GetJob(context.Background(), "nonexistent")

	assert.Nil(t, job)
	require.Error(t, err)

	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
	assert.Equal(t, http.StatusNotFound, svcErr.Status)
	mockRepo.AssertExpectations(t)
}

func TestDeletionService_GetJob_DBError(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	svc := newTestDeletionService(mockRepo, nil, nil, nil)

	mockRepo.On("GetJob", mock.Anything, "del-job-789").Return(nil, fmt.Errorf("timeout"))

	job, err := svc.GetJob(context.Background(), "del-job-789")

	assert.Nil(t, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	mockRepo.AssertExpectations(t)
}

// --- Tests: Password is never passed to repo (security) ---

func TestDeletionService_CreateJob_PasswordNotInJobRecord(t *testing.T) {
	mockRepo := new(mockDeletionJobRepository)
	mockExportRepo := new(mockExportJobRepository)
	authClient := buildValidAuthClient()
	mockEngine := &mockDeletionJobSubmitter{}

	now := time.Now().UTC().Truncate(time.Millisecond)
	expectedJob := &model.DeletionJob{
		ID:          "del-job-secure",
		UserID:      "target-user",
		AdminUserID: "admin-user",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	mockExportRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	mockRepo.On("GetInProgressJob", mock.Anything, "target-user").Return(nil, nil)
	// CreateJob is only called with userID and adminUserID: no password argument
	mockRepo.On("CreateJob", mock.Anything, "target-user", "admin-user").Return(expectedJob, nil)

	svc := newTestDeletionService(mockRepo, authClient, mockExportRepo, mockEngine)

	result, err := svc.CreateJob(context.Background(), "target-user", "admin-user", "super-secret-password")

	require.NoError(t, err)
	assert.NotNil(t, result.Job)
	// The repository CreateJob signature only accepts (ctx, userID, adminUserID)
	// This test confirms password is NOT passed to the persistence layer
	mockRepo.AssertCalled(t, "CreateJob", mock.Anything, "target-user", "admin-user")
}
