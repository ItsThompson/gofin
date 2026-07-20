package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
)

// --- UpdateProfile Tests ---

func TestUpdateProfile_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "new@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "newname").Return(nil, nil)
	repo.On("UpdateUser", ctx, "user-123", "newname", "new@example.com", "EUR").Return(&model.User{
		ID:       "user-123",
		Username: "newname",
		Email:    "new@example.com",
		Currency: "EUR",
		Role:     "user",
	}, nil)

	user, err := svc.UpdateProfile(ctx, "user-123", &model.UpdateProfileRequest{
		Username: "newname",
		Email:    "new@example.com",
		Currency: "EUR",
	})

	require.NoError(t, err)
	assert.Equal(t, "newname", user.Username)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, "EUR", user.Currency)
	repo.AssertExpectations(t)
}

func TestUpdateProfile_DuplicateEmail(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "taken@example.com").Return(&model.User{
		ID:    "other-user",
		Email: "taken@example.com",
	}, nil)

	_, err := svc.UpdateProfile(ctx, "user-123", &model.UpdateProfileRequest{
		Username: "myname",
		Email:    "taken@example.com",
		Currency: "USD",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateEmail, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}

func TestUpdateProfile_DuplicateUsername(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "me@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "takenuser").Return(&model.User{
		ID:       "other-user",
		Username: "takenuser",
	}, nil)

	_, err := svc.UpdateProfile(ctx, "user-123", &model.UpdateProfileRequest{
		Username: "takenuser",
		Email:    "me@example.com",
		Currency: "USD",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateUsername, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}

func TestUpdateProfile_SameEmailSameUser_Succeeds(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	// The user keeps their own email: should not be a conflict
	repo.On("GetUserByEmail", ctx, "me@example.com").Return(&model.User{
		ID:    "user-123",
		Email: "me@example.com",
	}, nil)
	repo.On("GetUserByUsername", ctx, "myname").Return(&model.User{
		ID:       "user-123",
		Username: "myname",
	}, nil)
	repo.On("UpdateUser", ctx, "user-123", "myname", "me@example.com", "USD").Return(&model.User{
		ID:       "user-123",
		Username: "myname",
		Email:    "me@example.com",
		Currency: "USD",
	}, nil)

	user, err := svc.UpdateProfile(ctx, "user-123", &model.UpdateProfileRequest{
		Username: "myname",
		Email:    "me@example.com",
		Currency: "USD",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
}

// TestUpdateProfile_OwnRecordMissing_Returns401 asserts that when the caller's
// own record is missing, UpdateProfile returns 401 UNAUTHORIZED, matching
// GetUserByID, RefreshToken, CompleteOnboarding, and ChangePassword.
func TestUpdateProfile_OwnRecordMissing_Returns401(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "me@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "myname").Return(nil, nil)
	// UpdateUser reports no matching row (own record vanished).
	repo.On("UpdateUser", ctx, "user-123", "myname", "me@example.com", "USD").Return(nil, nil)

	_, err := svc.UpdateProfile(ctx, "user-123", &model.UpdateProfileRequest{
		Username: "myname",
		Email:    "me@example.com",
		Currency: "USD",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeUnauthorized, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

// --- ChangePassword Tests ---

func TestChangePassword_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("OldPass123")
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:           "user-123",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hash,
		Role:         "user",
	}, nil)
	repo.On("UpdatePassword", ctx, "user-123", mock.AnythingOfType("string")).Return(nil)
	repo.On("RevokeAllUserTokens", ctx, "user-123").Return(nil)

	user, tokens, err := svc.ChangePassword(ctx, "user-123", &model.ChangePasswordRequest{
		CurrentPassword: "OldPass123",
		NewPassword:     "NewPass456",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	repo.AssertCalled(t, "UpdatePassword", ctx, "user-123", mock.AnythingOfType("string"))
	repo.AssertCalled(t, "RevokeAllUserTokens", ctx, "user-123")
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("CorrectPass1")
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:           "user-123",
		PasswordHash: hash,
	}, nil)

	_, _, err := svc.ChangePassword(ctx, "user-123", &model.ChangePasswordRequest{
		CurrentPassword: "WrongPass1",
		NewPassword:     "NewPass456",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrInvalidCredentials, apiErr.Code)
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("OldPass123")
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:           "user-123",
		PasswordHash: hash,
	}, nil)

	_, _, err := svc.ChangePassword(ctx, "user-123", &model.ChangePasswordRequest{
		CurrentPassword: "OldPass123",
		NewPassword:     "weak",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrWeakPassword, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestChangePassword_NewPasswordTooLong(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("OldPass123")
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:           "user-123",
		PasswordHash: hash,
	}, nil)

	// 73 bytes: exceeds bcrypt's 72-byte limit
	longPassword := "Ab1" + strings.Repeat("x", 70)
	_, _, err := svc.ChangePassword(ctx, "user-123", &model.ChangePasswordRequest{
		CurrentPassword: "OldPass123",
		NewPassword:     longPassword,
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrWeakPassword, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
	assert.Contains(t, apiErr.Message, "must not exceed 72 characters")
}

func TestChangePassword_TokenRevocation(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	hash, _ := svc.password.HashPassword("OldPass123")
	repo.On("GetUserByID", ctx, "user-123").Return(&model.User{
		ID:           "user-123",
		Username:     "testuser",
		PasswordHash: hash,
		Role:         "user",
	}, nil)
	repo.On("UpdatePassword", ctx, "user-123", mock.AnythingOfType("string")).Return(nil)
	repo.On("RevokeAllUserTokens", ctx, "user-123").Return(nil)

	_, _, err := svc.ChangePassword(ctx, "user-123", &model.ChangePasswordRequest{
		CurrentPassword: "OldPass123",
		NewPassword:     "NewPass456",
	})

	require.NoError(t, err)
	// RevokeAllUserTokens was called, which sets tokens_revoked_at on the user.
	// Any token with iat before that timestamp will be rejected by ValidateToken.
	repo.AssertCalled(t, "RevokeAllUserTokens", ctx, "user-123")
}
