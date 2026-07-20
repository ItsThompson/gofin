package service

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/stretchr/testify/mock"

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
