package handler

import (
	"context"
	"errors"
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
	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

func newTestGRPCHandler() (*GRPCHandler, *service.JWTService, *mockUserRepository) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
	return NewGRPCHandler(authSvc, logger), jwtSvc, repo
}

func newTestGRPCHandlerWithBlacklist() (*GRPCHandler, *mockUserRepository, *mockBlacklistRepository) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
	return NewGRPCHandler(authSvc, logger), repo, blacklistRepo
}

// TestIsMissingUser_ClassifiesWrappedTypedError locks in the C7 gRPC change:
// the handlers classify the service's "user not found" signal with errors.As,
// so a %w-wrapped *apierr.Error (401) is still recognized and mapped to
// codes.NotFound, while unrelated errors and non-401 apierr.Errors are not.
func TestIsMissingUser_ClassifiesWrappedTypedError(t *testing.T) {
	// Bare 401 (what GetUserByID returns for a missing user).
	assert.True(t, isMissingUser(apierr.Unauthorized("User not found")))

	// %w-wrapped 401 still classifies via errors.As.
	wrapped := fmt.Errorf("looking up user: %w", apierr.Unauthorized("User not found"))
	assert.True(t, isMissingUser(wrapped))

	// A 404 apierr.Error is a different case, not "missing user" for these RPCs.
	assert.False(t, isMissingUser(apierr.NotFound("Target user not found")))

	// A plain error is not classified.
	assert.False(t, isMissingUser(errors.New("db connection lost")))
}

func TestGRPCValidateToken_Success(t *testing.T) {
	handler, jwtSvc, repo := newTestGRPCHandler()

	// Token not revoked
	repo.On("GetTokensRevokedAt", mock.Anything, "user-123").Return(nil, nil)

	access, _, err := jwtSvc.GenerateTokenPair("user-123", "admin", "johndoe")
	require.NoError(t, err)

	resp, err := handler.ValidateToken(context.Background(), &pb.ValidateTokenRequest{
		AccessToken: access,
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", resp.UserId)
	assert.Equal(t, "admin", resp.Role)
	assert.Equal(t, "johndoe", resp.Username)
	assert.Empty(t, resp.AssumedBy)
}

func TestGRPCValidateToken_Invalid(t *testing.T) {
	handler, _, _ := newTestGRPCHandler()

	_, err := handler.ValidateToken(context.Background(), &pb.ValidateTokenRequest{
		AccessToken: "garbage",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGRPCGetUser_Success(t *testing.T) {
	handler, _, repo := newTestGRPCHandler()

	createdAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(&model.User{
		ID:                     "user-123",
		Username:               "johndoe",
		Email:                  "john@example.com",
		Role:                   "user",
		Currency:               "USD",
		HasCompletedOnboarding: true,
		CreatedAt:              createdAt,
	}, nil)

	resp, err := handler.GetUser(context.Background(), &pb.GetUserRequest{
		UserId: "user-123",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", resp.Id)
	assert.Equal(t, "johndoe", resp.Username)
	assert.Equal(t, "john@example.com", resp.Email)
	assert.Equal(t, "user", resp.Role)
	assert.Equal(t, "USD", resp.Currency)
	assert.True(t, resp.HasCompletedOnboarding)
	assert.Equal(t, "2026-01-15T10:00:00Z", resp.CreatedAt)
}

func TestGRPCGetUser_NotFound(t *testing.T) {
	handler, _, repo := newTestGRPCHandler()

	repo.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, nil)

	_, err := handler.GetUser(context.Background(), &pb.GetUserRequest{
		UserId: "nonexistent",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGRPCGetUser_EmptyUserID(t *testing.T) {
	handler, _, _ := newTestGRPCHandler()

	_, err := handler.GetUser(context.Background(), &pb.GetUserRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// --- VerifyPassword Tests ---

func TestGRPCVerifyPassword_Valid(t *testing.T) {
	handler, repo, _ := newTestGRPCHandlerWithBlacklist()

	// Hash for "CorrectPass1"
	pwdSvc := service.NewPasswordService(4)
	hash, err := pwdSvc.HashPassword("CorrectPass1")
	require.NoError(t, err)

	repo.On("GetUserByID", mock.Anything, "user-123").Return(&model.User{
		ID:           "user-123",
		Username:     "johndoe",
		PasswordHash: hash,
	}, nil)

	resp, err := handler.VerifyPassword(context.Background(), &pb.VerifyPasswordRequest{
		UserId:   "user-123",
		Password: "CorrectPass1",
	})

	require.NoError(t, err)
	assert.True(t, resp.Valid)
}

func TestGRPCVerifyPassword_Invalid(t *testing.T) {
	handler, repo, _ := newTestGRPCHandlerWithBlacklist()

	pwdSvc := service.NewPasswordService(4)
	hash, err := pwdSvc.HashPassword("CorrectPass1")
	require.NoError(t, err)

	repo.On("GetUserByID", mock.Anything, "user-123").Return(&model.User{
		ID:           "user-123",
		Username:     "johndoe",
		PasswordHash: hash,
	}, nil)

	resp, err := handler.VerifyPassword(context.Background(), &pb.VerifyPasswordRequest{
		UserId:   "user-123",
		Password: "WrongPassword1",
	})

	require.NoError(t, err)
	assert.False(t, resp.Valid)
}

func TestGRPCVerifyPassword_UserNotFound(t *testing.T) {
	handler, repo, _ := newTestGRPCHandlerWithBlacklist()

	repo.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, nil)

	_, err := handler.VerifyPassword(context.Background(), &pb.VerifyPasswordRequest{
		UserId:   "nonexistent",
		Password: "SomePass123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGRPCVerifyPassword_EmptyUserID(t *testing.T) {
	handler, _, _ := newTestGRPCHandlerWithBlacklist()

	_, err := handler.VerifyPassword(context.Background(), &pb.VerifyPasswordRequest{
		Password: "SomePass123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id")
}

func TestGRPCVerifyPassword_EmptyPassword(t *testing.T) {
	handler, _, _ := newTestGRPCHandlerWithBlacklist()

	_, err := handler.VerifyPassword(context.Background(), &pb.VerifyPasswordRequest{
		UserId: "user-123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "password")
}

// --- DeleteUserData Tests ---

func TestGRPCDeleteUserData_Success(t *testing.T) {
	handler, repo, blacklistRepo := newTestGRPCHandlerWithBlacklist()

	blacklistRepo.On("DeleteByUserID", mock.Anything, "user-123").Return(nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(nil)

	resp, err := handler.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{
		UserId: "user-123",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	blacklistRepo.AssertCalled(t, "DeleteByUserID", mock.Anything, "user-123")
	repo.AssertCalled(t, "DeleteUser", mock.Anything, "user-123")
}

func TestGRPCDeleteUserData_NonexistentUser(t *testing.T) {
	// Idempotent: returns success even if user does not exist (0 rows affected)
	handler, repo, blacklistRepo := newTestGRPCHandlerWithBlacklist()

	blacklistRepo.On("DeleteByUserID", mock.Anything, "nonexistent").Return(nil)
	repo.On("DeleteUser", mock.Anything, "nonexistent").Return(nil)

	resp, err := handler.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{
		UserId: "nonexistent",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPCDeleteUserData_EmptyUserID(t *testing.T) {
	handler, _, _ := newTestGRPCHandlerWithBlacklist()

	_, err := handler.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id")
}

func TestGRPCDeleteUserData_BlacklistDeleteFails(t *testing.T) {
	handler, _, blacklistRepo := newTestGRPCHandlerWithBlacklist()

	blacklistRepo.On("DeleteByUserID", mock.Anything, "user-123").Return(fmt.Errorf("db connection lost"))

	_, err := handler.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{
		UserId: "user-123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "refresh tokens")
}

func TestGRPCDeleteUserData_UserDeleteFails(t *testing.T) {
	handler, repo, blacklistRepo := newTestGRPCHandlerWithBlacklist()

	blacklistRepo.On("DeleteByUserID", mock.Anything, "user-123").Return(nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(fmt.Errorf("db connection lost"))

	_, err := handler.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{
		UserId: "user-123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "delete user")
}
