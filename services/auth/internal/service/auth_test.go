package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
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

func newTestAuthService(repo *mockUserRepository) *AuthService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := NewJWTService("test-secret")
	pwdSvc := NewPasswordService(4) // Low cost for fast tests
	return NewAuthService(repo, jwtSvc, pwdSvc, logger)
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

	jwtSvc := NewJWTService("test-secret")
	access, _, err := jwtSvc.GenerateTokenPair("user-123", "admin", "johndoe")
	require.NoError(t, err)

	result, err := svc.ValidateToken(access)
	require.NoError(t, err)
	assert.Equal(t, "user-123", result.UserID)
	assert.Equal(t, "admin", result.Role)
	assert.Equal(t, "johndoe", result.Username)
}

func TestValidateToken_Invalid(t *testing.T) {
	repo := new(mockUserRepository)
	svc := newTestAuthService(repo)

	_, err := svc.ValidateToken("garbage-token")
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, model.ErrUnauthorized, authErr.Code)
}
