package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
)

// mockUserRepository implements repository.UserRepository for testing.
type mockUserRepository struct {
	mock.Mock
}

func (m *mockUserRepository) CreateUser(ctx context.Context, username, email, passwordHash, role, currency string) (*model.User, error) {
	args := m.Called(ctx, username, email, passwordHash, role, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) CompleteOnboarding(ctx context.Context, userID string, currency string) (*model.User, error) {
	args := m.Called(ctx, userID, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) ListAllUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *mockUserRepository) UpdateUser(ctx context.Context, userID, username, email, currency string) (*model.User, error) {
	args := m.Called(ctx, userID, username, email, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockUserRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepository) GetTokensRevokedAt(ctx context.Context, userID string) (*time.Time, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *mockUserRepository) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func newTestAuthService(repo *mockUserRepository) *AuthService {
	blacklistRepo := new(mockBlacklistRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4) // Low cost for fast tests
	return NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
}

func newTestAuthServiceWithBlacklist(repo *mockUserRepository, blacklistRepo *mockBlacklistRepository) *AuthService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4)
	return NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)
}

// mockBlacklistRepository implements repository.BlacklistRepository for testing.
type mockBlacklistRepository struct {
	mock.Mock
}

func (m *mockBlacklistRepository) ConsumeToken(ctx context.Context, jti, userID string, expiresAt time.Time) (bool, error) {
	args := m.Called(ctx, jti, userID, expiresAt)
	return args.Bool(0), args.Error(1)
}

func (m *mockBlacklistRepository) BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	args := m.Called(ctx, jti, userID, expiresAt)
	return args.Error(0)
}

func (m *mockBlacklistRepository) CleanupExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockBlacklistRepository) DeleteByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrWeakPassword, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrWeakPassword, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
	assert.Contains(t, authErr.Message, "must not exceed 72 characters")
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateEmail, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateUsername, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateEmail, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateUsername, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
}

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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrInvalidCredentials, authErr.Code)
	assert.Equal(t, 401, authErr.Status)
	// Must not hint at which field is wrong
	assert.Equal(t, "Invalid email or password", authErr.Message)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrInvalidCredentials, authErr.Code)
	// Same message as wrong password: no hint about whether user exists
	assert.Equal(t, "Invalid email or password", authErr.Message)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
	assert.Contains(t, authErr.Message, "revoked")
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
	assert.Equal(t, 401, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
	assert.Equal(t, 401, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
	assert.Equal(t, 401, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrValidationError, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
}

func TestAssumeIdentity_TargetNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "nonexistent").Return(nil, nil)

	_, _, err := svc.AssumeIdentity(ctx, "admin-123", "nonexistent")

	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrNotFound, authErr.Code)
	assert.Equal(t, 404, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrValidationError, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
}

func TestRestoreIdentity_AdminNotFound(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)
	ctx := context.Background()

	repo.On("GetUserByID", ctx, "deleted-admin").Return(nil, nil)

	_, _, err := svc.RestoreIdentity(ctx, "deleted-admin")

	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrNotFound, authErr.Code)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrForbidden, authErr.Code)
	assert.Equal(t, 403, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateEmail, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrDuplicateUsername, authErr.Code)
	assert.Equal(t, 409, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrInvalidCredentials, authErr.Code)
	assert.Equal(t, 401, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrWeakPassword, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
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
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrWeakPassword, authErr.Code)
	assert.Equal(t, 400, authErr.Status)
	assert.Contains(t, authErr.Message, "must not exceed 72 characters")
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

