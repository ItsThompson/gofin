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
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
)

// --- Registration Tests ---

func TestRegister_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "test@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "testuser").Return(nil, nil)
	repo.On("CreateUser", ctx, "testuser", "test@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(&model.User{
			ID:       "user-123",
			Username: "testuser",
			Email:    "test@example.com",
			Role:     "user",
			Currency: "USD",
		}, nil)

	user, tokens, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "ValidPass1",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	repo.AssertExpectations(t)
}

func TestRegister_WeakPassword(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "weak",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrWeakPassword, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestRegister_PasswordTooLong(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	// 73 bytes: exceeds bcrypt's 72-byte limit
	longPassword := "Ab1" + strings.Repeat("x", 70)
	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: longPassword,
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrWeakPassword, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
	assert.Contains(t, apiErr.Message, "must not exceed 72 characters")
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "taken@example.com").Return(&model.User{
		ID:    "existing-user",
		Email: "taken@example.com",
	}, nil)

	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "newuser",
		Email:    "taken@example.com",
		Password: "ValidPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateEmail, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "new@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "takenuser").Return(&model.User{
		ID:       "existing-user",
		Username: "takenuser",
	}, nil)

	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "takenuser",
		Email:    "new@example.com",
		Password: "ValidPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateUsername, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}

func TestRegister_EmailNormalization(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	// Email should be lowercased and trimmed
	repo.On("GetUserByEmail", ctx, "test@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "testuser").Return(nil, nil)
	repo.On("CreateUser", ctx, "testuser", "test@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(&model.User{
			ID:       "user-123",
			Username: "testuser",
			Email:    "test@example.com",
			Role:     "user",
		}, nil)

	user, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: " testuser ",
		Email:    " Test@Example.COM ",
		Password: "ValidPass1",
	})

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	repo.AssertExpectations(t)
}

func TestRegister_DuplicateEmailFromConstraint(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	// Both uniqueness checks pass (TOCTOU race), but INSERT hits constraint
	repo.On("GetUserByEmail", ctx, "race@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "raceuser").Return(nil, nil)
	repo.On("CreateUser", ctx, "raceuser", "race@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(nil, &repository.DuplicateError{Constraint: "users_email_key"})

	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "raceuser",
		Email:    "race@example.com",
		Password: "ValidPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateEmail, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}

func TestRegister_DuplicateUsernameFromConstraint(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "unique@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", ctx, "raceuser").Return(nil, nil)
	repo.On("CreateUser", ctx, "raceuser", "unique@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(nil, &repository.DuplicateError{Constraint: "users_username_key"})

	_, _, err := svc.Register(ctx, &model.RegisterRequest{
		Username: "raceuser",
		Email:    "unique@example.com",
		Password: "ValidPass1",
	})

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, model.ErrDuplicateUsername, apiErr.Code)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
}
