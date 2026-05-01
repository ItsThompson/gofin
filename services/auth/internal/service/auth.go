package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
)

// AuthService contains the business logic for authentication operations.
type AuthService struct {
	repo          repository.UserRepository
	blacklistRepo repository.BlacklistRepository
	jwt           *JWTService
	password      *PasswordService
	logger        *slog.Logger
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	repo repository.UserRepository,
	blacklistRepo repository.BlacklistRepository,
	jwt *JWTService,
	password *PasswordService,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		repo:          repo,
		blacklistRepo: blacklistRepo,
		jwt:           jwt,
		password:      password,
		logger:        logger,
	}
}

// Register creates a new user after validating input, checking uniqueness,
// and hashing the password. Returns the user and a token pair.
func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.User, *model.TokenPair, error) {

	// Validate password strength
	if err := ValidatePasswordStrength(req.Password); err != nil {
		return nil, nil, &AuthError{
			Code:    model.ErrWeakPassword,
			Message: err.Error(),
			Status:  400,
		}
	}

	// Normalize email to lowercase
	email := strings.ToLower(strings.TrimSpace(req.Email))
	username := strings.TrimSpace(req.Username)

	// Check for duplicate email
	existingByEmail, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("checking email uniqueness: %w", err)
	}
	if existingByEmail != nil {
		return nil, nil, &AuthError{
			Code:    model.ErrDuplicateEmail,
			Message: "An account with this email already exists",
			Status:  409,
		}
	}

	// Check for duplicate username
	existingByUsername, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, nil, fmt.Errorf("checking username uniqueness: %w", err)
	}
	if existingByUsername != nil {
		return nil, nil, &AuthError{
			Code:    model.ErrDuplicateUsername,
			Message: "This username is already taken",
			Status:  409,
		}
	}

	// Hash password
	hash, err := s.password.HashPassword(req.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing password: %w", err)
	}

	// Create user
	user, err := s.repo.CreateUser(ctx, username, email, hash, "user", "USD")
	if err != nil {
		var dupErr *repository.DuplicateError
		if errors.As(err, &dupErr) {
			if strings.Contains(dupErr.Constraint, "email") {
				return nil, nil, &AuthError{
					Code:    model.ErrDuplicateEmail,
					Message: "An account with this email already exists",
					Status:  409,
				}
			}
			return nil, nil, &AuthError{
				Code:    model.ErrDuplicateUsername,
				Message: "This username is already taken",
				Status:  409,
			}
		}
		return nil, nil, fmt.Errorf("creating user: %w", err)
	}

	s.logger.Info("user registered",
		slog.String("method", "Register"),
		slog.String("user_id", user.ID),
	)

	// Generate tokens
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Role, user.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	return user, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Login authenticates a user by email and password.
// Returns the user and a token pair on success.
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.User, *model.TokenPair, error) {

	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Look up user by email
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up user: %w", err)
	}

	// Generic error for both "user not found" and "wrong password" (no field hints)
	if user == nil {
		return nil, nil, &AuthError{
			Code:    model.ErrInvalidCredentials,
			Message: "Invalid email or password",
			Status:  401,
		}
	}

	// Verify password
	if !s.password.CheckPassword(req.Password, user.PasswordHash) {
		return nil, nil, &AuthError{
			Code:    model.ErrInvalidCredentials,
			Message: "Invalid email or password",
			Status:  401,
		}
	}

	s.logger.Info("user logged in",
		slog.String("method", "Login"),
		slog.String("user_id", user.ID),
	)

	// Generate tokens
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Role, user.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	return user, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateToken verifies an access token and returns the user identity.
func (s *AuthService) ValidateToken(tokenString string) (*model.ValidateTokenResult, error) {
	claims, err := s.jwt.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Please log in again",
			Status:  401,
		}
	}

	return &model.ValidateTokenResult{
		UserID:    claims.Subject,
		Role:      claims.Role,
		Username:  claims.Username,
		AssumedBy: claims.AssumedBy,
	}, nil
}

// GetUserByID looks up a user by their ID.
func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}
	if user == nil {
		return nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "User not found",
			Status:  401,
		}
	}
	return user, nil
}

// AuthError is a typed error that carries an HTTP status code and error code.
type AuthError struct {
	Code    string
	Message string
	Status  int
}

func (e *AuthError) Error() string {
	return e.Message
}

// RefreshToken validates a refresh token, blacklists the old one, and generates
// a new access + refresh token pair (refresh token rotation).
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*model.User, *model.TokenPair, error) {
	// Validate the refresh token JWT
	claims, err := s.jwt.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return nil, nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Invalid or expired refresh token",
			Status:  401,
		}
	}

	// Check if this token has been blacklisted
	blacklisted, err := s.blacklistRepo.IsTokenBlacklisted(ctx, claims.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("checking blacklist: %w", err)
	}
	if blacklisted {
		s.logger.Warn("blacklisted refresh token used",
			slog.String("method", "RefreshToken"),
			slog.String("jti", claims.ID),
			slog.String("user_id", claims.Subject),
		)
		return nil, nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Refresh token has been revoked",
			Status:  401,
		}
	}

	// Look up the user
	user, err := s.repo.GetUserByID(ctx, claims.Subject)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up user for refresh: %w", err)
	}
	if user == nil {
		return nil, nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "User not found",
			Status:  401,
		}
	}

	// Blacklist the old refresh token
	expiresAt := claims.ExpiresAt.Time
	if err := s.blacklistRepo.BlacklistToken(ctx, claims.ID, user.ID, expiresAt); err != nil {
		return nil, nil, fmt.Errorf("blacklisting old token: %w", err)
	}

	// Generate new token pair
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Role, user.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	s.logger.Info("token refreshed",
		slog.String("method", "RefreshToken"),
		slog.String("user_id", user.ID),
	)

	// Best-effort cleanup of expired blacklist entries
	go func() {
		if err := s.blacklistRepo.CleanupExpired(context.Background()); err != nil {
			s.logger.Error("failed to cleanup expired blacklist entries",
				slog.String("error", err.Error()),
			)
		}
	}()

	return user, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Logout blacklists the current refresh token so it cannot be reused.
func (s *AuthService) Logout(ctx context.Context, refreshTokenString string) error {
	// Parse the refresh token to extract the JTI.
	// We still blacklist even if the token is expired: prevents reuse of a
	// recently-expired token during clock skew.
	claims, err := s.jwt.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		// Token is invalid or expired: nothing to blacklist.
		// The logout still succeeds (cookies will be cleared by the handler).
		s.logger.Info("logout with invalid refresh token, skipping blacklist",
			slog.String("method", "Logout"),
			slog.String("error", err.Error()),
		)
		return nil
	}

	expiresAt := claims.ExpiresAt.Time
	if err := s.blacklistRepo.BlacklistToken(ctx, claims.ID, claims.Subject, expiresAt); err != nil {
		return fmt.Errorf("blacklisting token on logout: %w", err)
	}

	s.logger.Info("user logged out",
		slog.String("method", "Logout"),
		slog.String("user_id", claims.Subject),
	)

	return nil
}
