package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thompsnt/gofin/services/auth/internal/model"
	"github.com/thompsnt/gofin/services/auth/internal/repository"
)

// AuthService contains the business logic for authentication operations.
type AuthService struct {
	repo     repository.UserRepository
	jwt      *JWTService
	password *PasswordService
	logger   *slog.Logger
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	repo repository.UserRepository,
	jwt *JWTService,
	password *PasswordService,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		repo:     repo,
		jwt:      jwt,
		password: password,
		logger:   logger,
	}
}

// Register creates a new user after validating input, checking uniqueness,
// and hashing the password. Returns the user and a token pair.
func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.User, *model.TokenPair, error) {
	start := time.Now()
	defer func() {
		s.logger.Info("register completed",
			slog.String("method", "Register"),
			slog.Duration("duration_ms", time.Since(start)),
		)
	}()

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
	start := time.Now()
	defer func() {
		s.logger.Info("login completed",
			slog.String("method", "Login"),
			slog.Duration("duration_ms", time.Since(start)),
		)
	}()

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

// AuthError is a typed error that carries an HTTP status code and error code.
type AuthError struct {
	Code    string
	Message string
	Status  int
}

func (e *AuthError) Error() string {
	return e.Message
}
