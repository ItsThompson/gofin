package handler

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func TestGRPCStubs_ReturnUnimplemented(t *testing.T) {
	handler, _, _ := newTestGRPCHandler()
	ctx := context.Background()

	_, err := handler.RefreshToken(ctx, &pb.RefreshTokenRequest{})
	assertUnimplemented(t, err)

	_, err = handler.Logout(ctx, &pb.LogoutRequest{})
	assertUnimplemented(t, err)

	_, err = handler.AssumeIdentity(ctx, &pb.AssumeIdentityRequest{})
	assertUnimplemented(t, err)

	_, err = handler.RestoreIdentity(ctx, &pb.RestoreIdentityRequest{})
	assertUnimplemented(t, err)

	_, err = handler.ListUsers(ctx, &pb.ListUsersRequest{})
	assertUnimplemented(t, err)

	_, err = handler.GetUser(ctx, &pb.GetUserRequest{})
	assertUnimplemented(t, err)

	_, err = handler.UpdateUser(ctx, &pb.UpdateUserRequest{})
	assertUnimplemented(t, err)

	_, err = handler.ChangePassword(ctx, &pb.ChangePasswordRequest{})
	assertUnimplemented(t, err)
}

func assertUnimplemented(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}
