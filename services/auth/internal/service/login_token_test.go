package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
)

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	// Pre-hash a password to use in the mock
	hash, _ := svc.password.HashPassword("ValidPass1")
	repo.On("GetUserByEmail", ctx, "test@example.com").Return(&model.User{
		ID:           "user-123",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hash,
		Role:         "user",
	}, nil)

	user, tokens, err := svc.Login(ctx, &model.LoginRequest{
		Email:    "test@example.com",
		Password: "ValidPass1",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("CorrectPass1")
	repo.On("GetUserByEmail", ctx, "test@example.com").Return(&model.User{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: hash,
	}, nil)

	_, _, err := svc.Login(ctx, &model.LoginRequest{
		Email:    "test@example.com",
		Password: "WrongPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrInvalidCredentials, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
	// Must not hint at which field is wrong
	assert.Equal(t, "Invalid email or password", apiErr.Message)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "nobody@example.com").Return(nil, nil)

	_, _, err := svc.Login(ctx, &model.LoginRequest{
		Email:    "nobody@example.com",
		Password: "ValidPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrInvalidCredentials, apiErr.Code)
	// Same message as wrong password: no hint about whether user exists
	assert.Equal(t, "Invalid email or password", apiErr.Message)
}

// --- ValidateToken Tests ---

func TestValidateToken_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	access, _, err := jwtSvc.GenerateTokenPair("user-123", "admin", "johndoe")
	require.NoError(t, err)

	// Token not revoked
	repo.On("GetTokensRevokedAt", ctx, "user-123").Return(nil, nil)

	result, err := svc.ValidateToken(ctx, access)
	require.NoError(t, err)
	assert.Equal(t, "user-123", result.UserID)
	assert.Equal(t, "admin", result.Role)
	assert.Equal(t, "johndoe", result.Username)
}

func TestValidateToken_Invalid(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	_, err := svc.ValidateToken(ctx, "garbage-token")
	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
}

func TestValidateToken_RevokedToken(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	access, _, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	// Token revoked well AFTER the token was issued: clear revocation case
	revokedTime := time.Now().Add(1 * time.Second)
	repo.On("GetTokensRevokedAt", ctx, "user-123").Return(&revokedTime, nil)

	_, err = svc.ValidateToken(ctx, access)
	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
	assert.Contains(t, apiErr.Message, "revoked")
}

func TestValidateToken_SameSecondGracePeriod(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	access, _, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	// Parse the token to get its actual iat (whole seconds)
	claims, err := jwtSvc.ValidateAccessToken(access)
	require.NoError(t, err)
	tokenIat := claims.IssuedAt.Time // e.g., 10:00:05.000000000

	// Simulate the ChangePassword race: revocation happens within the
	// same second as token issuance, but with a microsecond offset.
	// PostgreSQL stores microsecond precision, JWT iat is whole seconds.
	// The truncation fix should treat these as the same second.
	revokedTime := tokenIat.Add(500 * time.Millisecond) // same second, 500ms after iat
	repo.On("GetTokensRevokedAt", ctx, "user-123").Return(&revokedTime, nil)

	result, err := svc.ValidateToken(ctx, access)
	require.NoError(t, err, "token issued in same second as revocation should not be rejected")
	assert.Equal(t, "user-123", result.UserID)
}

// --- RefreshToken Tests ---

func TestRefreshToken_Success(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	// Generate a valid refresh token
	jwtSvc := NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	// Parse it to get the JTI for mock expectations
	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("ConsumeToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(true, nil)
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:       "user-123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
		Currency: "USD",
	}, nil)

	user, tokens, err := svc.RefreshToken(ctx, refreshToken)

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	// New tokens should be different from the original
	assert.NotEqual(t, refreshToken, tokens.RefreshToken)
	blacklistRepo.AssertCalled(t, "ConsumeToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time"))
}

func TestRefreshToken_BlacklistedToken(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	// ConsumeToken returns (false, nil) meaning token was already consumed
	blacklistRepo.On("ConsumeToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(false, nil)

	_, _, err = svc.RefreshToken(ctx, refreshToken)

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestRefreshToken_ConsumeError(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	// ConsumeToken returns (false, err) meaning a database error occurred
	dbErr := fmt.Errorf("connection refused")
	blacklistRepo.On("ConsumeToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(false, dbErr)

	_, _, err = svc.RefreshToken(ctx, refreshToken)

	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	assert.Contains(t, err.Error(), "consuming refresh token")
}

func TestRefreshToken_ExpiredToken(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	_, _, err := svc.RefreshToken(ctx, "expired-or-invalid-token")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestRefreshToken_UserNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("deleted-user", "user", "ghost")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("ConsumeToken", ctx, refreshClaims.ID, "deleted-user", mock.AnythingOfType("time.Time")).Return(true, nil)
	repo.On("GetUserByID", ctx, "deleted-user").Return(nil, nil)

	_, _, err = svc.RefreshToken(ctx, refreshToken)

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

// --- Logout Tests ---

func TestLogout_Success(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	jwtSvc := NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("BlacklistToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(nil)

	err = svc.Logout(ctx, refreshToken)

	require.NoError(t, err)
	blacklistRepo.AssertCalled(t, "BlacklistToken", ctx, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time"))
}

func TestLogout_InvalidToken_StillSucceeds(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	svc := newTestAuthServiceWithBlacklist(repo, blacklistRepo)
	ctx := context.Background()

	// Logout with an invalid/expired token should still succeed
	// (cookies will be cleared regardless)
	err := svc.Logout(ctx, "invalid-token")

	require.NoError(t, err)
	// BlacklistToken should NOT have been called since we couldn't parse the token
	blacklistRepo.AssertNotCalled(t, "BlacklistToken")
}
