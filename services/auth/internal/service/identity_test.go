package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
)

// --- ListUsers Tests ---

func TestListUsers_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("ListAllUsers", ctx).Return([]*model.User{
		{ID: "user-1", Username: "alice", Email: "alice@example.com", Role: "user"},
		{ID: "admin-1", Username: "admin", Email: "admin@example.com", Role: "admin"},
	}, nil)

	users, err := svc.ListUsers(ctx)

	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "alice", users[0].Username)
	assert.Equal(t, "admin", users[1].Username)
	repo.AssertExpectations(t)
}

func TestListUsers_Empty(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("ListAllUsers", ctx).Return([]*model.User{}, nil)

	users, err := svc.ListUsers(ctx)

	require.NoError(t, err)
	assert.Len(t, users, 0)
}

// --- AssumeIdentity Tests ---

func TestAssumeIdentity_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "target-456").Return(&model.User{
		ID:       "target-456",
		Username: "targetuser",
		Email:    "target@example.com",
		Role:     "user",
	}, nil)

	user, tokens, err := svc.AssumeIdentity(ctx, "admin-123", "target-456")

	require.NoError(t, err)
	assert.Equal(t, "target-456", user.ID)
	assert.Equal(t, "targetuser", user.Username)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)

	// Verify the access token has the assumedBy claim
	jwtSvc := NewJWTService("test-secret")
	claims, err := jwtSvc.ValidateAccessToken(tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "target-456", claims.Subject)
	assert.Equal(t, "user", claims.Role)
	assert.Equal(t, "admin-123", claims.AssumedBy)
	repo.AssertExpectations(t)
}

func TestAssumeIdentity_CannotAssumeSelf(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	_, _, err := svc.AssumeIdentity(ctx, "admin-123", "admin-123")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeValidation, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestAssumeIdentity_TargetNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "nonexistent").Return(nil, nil)

	_, _, err := svc.AssumeIdentity(ctx, "admin-123", "nonexistent")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeNotFound, apiErr.Code)
	assert.Equal(t, http.StatusNotFound, apiErr.Status)
}

// --- RestoreIdentity Tests ---

func TestRestoreIdentity_Success(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "admin-123").Return(&model.User{
		ID:       "admin-123",
		Username: "admin",
		Email:    "admin@example.com",
		Role:     "admin",
	}, nil)

	user, tokens, err := svc.RestoreIdentity(ctx, "admin-123")

	require.NoError(t, err)
	assert.Equal(t, "admin-123", user.ID)
	assert.Equal(t, "admin", user.Username)
	assert.NotEmpty(t, tokens.AccessToken)

	// Verify the restored token does NOT have an assumedBy claim
	jwtSvc := NewJWTService("test-secret")
	claims, err := jwtSvc.ValidateAccessToken(tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "admin-123", claims.Subject)
	assert.Equal(t, "admin", claims.Role)
	assert.Empty(t, claims.AssumedBy)
	repo.AssertExpectations(t)
}

func TestRestoreIdentity_EmptyAssumedBy(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	_, _, err := svc.RestoreIdentity(ctx, "")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeValidation, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestRestoreIdentity_AdminNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "deleted-admin").Return(nil, nil)

	_, _, err := svc.RestoreIdentity(ctx, "deleted-admin")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeNotFound, apiErr.Code)
}

func TestRestoreIdentity_UserNotAdmin(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "regular-user").Return(&model.User{
		ID:   "regular-user",
		Role: "user",
	}, nil)

	_, _, err := svc.RestoreIdentity(ctx, "regular-user")

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeForbidden, apiErr.Code)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}

// --- SeedAdmin Tests ---

func TestSeedAdmin_CreatesNewAdmin(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByUsername", ctx, "admin").Return(nil, nil)
	repo.On("CreateUser", ctx, "admin", "admin@gofin.local", mock.AnythingOfType("string"), "admin", "USD").
		Return(&model.User{
			ID:       "admin-1",
			Username: "admin",
			Email:    "admin@gofin.local",
			Role:     "admin",
		}, nil)
	repo.On("CompleteOnboarding", ctx, "admin-1", "USD").
		Return(&model.User{
			ID:                     "admin-1",
			Username:               "admin",
			Email:                  "admin@gofin.local",
			Role:                   "admin",
			HasCompletedOnboarding: true,
		}, nil)

	err := svc.SeedAdmin(ctx, "admin", "admin@gofin.local", "Admin1234!")

	require.NoError(t, err)
	repo.AssertCalled(t, "CreateUser", ctx, "admin", "admin@gofin.local", mock.AnythingOfType("string"), "admin", "USD")
}

func TestSeedAdmin_IdempotentSkipsExisting(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByUsername", ctx, "admin").Return(&model.User{
		ID:       "admin-1",
		Username: "admin",
		Role:     "admin",
	}, nil)

	err := svc.SeedAdmin(ctx, "admin", "admin@gofin.local", "Admin1234!")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "CreateUser")
}
