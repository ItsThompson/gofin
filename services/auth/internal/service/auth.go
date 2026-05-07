package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
// Checks both JWT validity and the tokens_revoked_at timestamp on the user record.
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*model.ValidateTokenResult, error) {
	claims, err := s.jwt.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Please log in again",
			Status:  401,
		}
	}

	// Check if user's tokens have been revoked (e.g., after password change)
	revokedAt, err := s.repo.GetTokensRevokedAt(ctx, claims.Subject)
	if err != nil {
		s.logger.Error("failed to check token revocation",
			slog.String("user_id", claims.Subject),
			slog.String("error", err.Error()),
		)
		// Fail open: if we can't check revocation, still reject to be safe
		return nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Unable to validate token",
			Status:  401,
		}
	}

	if revokedAt != nil && claims.IssuedAt != nil {
		tokenIssuedAt := claims.IssuedAt.Time
		// Truncate revokedAt to second precision for comparison. JWT iat
		// is Unix seconds (no sub-second component), but PostgreSQL
		// stores tokens_revoked_at with microsecond precision. Without
		// truncation, a token issued in the same second as the revocation
		// would be incorrectly rejected because its iat (whole second) is
		// before the microsecond-precise revocation timestamp.
		revokedAtTruncated := revokedAt.Truncate(time.Second)
		if tokenIssuedAt.Before(revokedAtTruncated) {
			return nil, &AuthError{
				Code:    model.ErrUnauthorized,
				Message: "Token has been revoked. Please log in again.",
				Status:  401,
			}
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

// CompleteOnboarding marks the user's onboarding as complete and updates their currency.
func (s *AuthService) CompleteOnboarding(ctx context.Context, userID string, currency string) (*model.User, error) {
	user, err := s.repo.CompleteOnboarding(ctx, userID, currency)
	if err != nil {
		return nil, fmt.Errorf("completing onboarding: %w", err)
	}
	if user == nil {
		return nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "User not found",
			Status:  401,
		}
	}

	s.logger.Info("onboarding completed",
		slog.String("method", "CompleteOnboarding"),
		slog.String("user_id", user.ID),
		slog.String("currency", currency),
	)

	return user, nil
}

// UpdateProfile updates the user's username, email, and currency.
// Validates uniqueness for both username and email.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req *model.UpdateProfileRequest) (*model.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	username := strings.TrimSpace(req.Username)
	currency := strings.TrimSpace(req.Currency)

	// Check for duplicate email (exclude current user)
	existingByEmail, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("checking email uniqueness: %w", err)
	}
	if existingByEmail != nil && existingByEmail.ID != userID {
		return nil, &AuthError{
			Code:    model.ErrDuplicateEmail,
			Message: "An account with this email already exists",
			Status:  409,
		}
	}

	// Check for duplicate username (exclude current user)
	existingByUsername, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("checking username uniqueness: %w", err)
	}
	if existingByUsername != nil && existingByUsername.ID != userID {
		return nil, &AuthError{
			Code:    model.ErrDuplicateUsername,
			Message: "This username is already taken",
			Status:  409,
		}
	}

	user, err := s.repo.UpdateUser(ctx, userID, username, email, currency)
	if err != nil {
		var dupErr *repository.DuplicateError
		if errors.As(err, &dupErr) {
			if strings.Contains(dupErr.Constraint, "email") {
				return nil, &AuthError{
					Code:    model.ErrDuplicateEmail,
					Message: "An account with this email already exists",
					Status:  409,
				}
			}
			return nil, &AuthError{
				Code:    model.ErrDuplicateUsername,
				Message: "This username is already taken",
				Status:  409,
			}
		}
		return nil, fmt.Errorf("updating user: %w", err)
	}
	if user == nil {
		return nil, &AuthError{
			Code:    model.ErrNotFound,
			Message: "User not found",
			Status:  404,
		}
	}

	s.logger.Info("profile updated",
		slog.String("method", "UpdateProfile"),
		slog.String("user_id", user.ID),
	)

	return user, nil
}

// ListUsers returns all registered users. Admin-only.
func (s *AuthService) ListUsers(ctx context.Context) ([]*model.User, error) {
	users, err := s.repo.ListAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	s.logger.Info("listed all users",
		slog.String("method", "ListUsers"),
		slog.Int("count", len(users)),
	)

	return users, nil
}

// AssumeIdentity generates a new token pair for the target user with the
// assumedBy claim set to the admin's user ID. The caller must verify the
// admin role before calling this method.
func (s *AuthService) AssumeIdentity(ctx context.Context, adminUserID, targetUserID string) (*model.User, *model.TokenPair, error) {
	if adminUserID == targetUserID {
		return nil, nil, &AuthError{
			Code:    model.ErrValidationError,
			Message: "Cannot assume your own identity",
			Status:  400,
		}
	}

	targetUser, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up target user: %w", err)
	}
	if targetUser == nil {
		return nil, nil, &AuthError{
			Code:    model.ErrNotFound,
			Message: "Target user not found",
			Status:  404,
		}
	}

	accessToken, refreshToken, err := s.jwt.GenerateTokenPairWithAssumedBy(
		targetUser.ID, targetUser.Role, targetUser.Username, adminUserID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("generating assumed tokens: %w", err)
	}

	s.logger.Info("identity assumed",
		slog.String("method", "AssumeIdentity"),
		slog.String("admin_user_id", adminUserID),
		slog.String("target_user_id", targetUser.ID),
	)

	return targetUser, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RestoreIdentity reads the assumedBy claim from the current access token,
// looks up the original admin user, and generates fresh tokens for that admin.
func (s *AuthService) RestoreIdentity(ctx context.Context, assumedByUserID string) (*model.User, *model.TokenPair, error) {
	if assumedByUserID == "" {
		return nil, nil, &AuthError{
			Code:    model.ErrValidationError,
			Message: "No assumed identity to restore",
			Status:  400,
		}
	}

	adminUser, err := s.repo.GetUserByID(ctx, assumedByUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up admin user: %w", err)
	}
	if adminUser == nil {
		return nil, nil, &AuthError{
			Code:    model.ErrNotFound,
			Message: "Admin user not found",
			Status:  404,
		}
	}

	if adminUser.Role != "admin" {
		return nil, nil, &AuthError{
			Code:    model.ErrForbidden,
			Message: "Assumed-by user is not an admin",
			Status:  403,
		}
	}

	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(
		adminUser.ID, adminUser.Role, adminUser.Username,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("generating admin tokens: %w", err)
	}

	s.logger.Info("identity restored",
		slog.String("method", "RestoreIdentity"),
		slog.String("admin_user_id", adminUser.ID),
	)

	return adminUser, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// SeedAdmin creates an admin user if one doesn't already exist.
// Idempotent: skips creation if a user with the given username already exists.
func (s *AuthService) SeedAdmin(ctx context.Context, username, email, password string) error {
	existing, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("checking existing admin: %w", err)
	}
	if existing != nil {
		s.logger.Info("admin user already exists, skipping seed",
			slog.String("method", "SeedAdmin"),
			slog.String("username", username),
		)
		return nil
	}

	hash, err := s.password.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, username, email, hash, "admin", "USD")
	if err != nil {
		return fmt.Errorf("creating admin user: %w", err)
	}

	// Mark onboarding as complete so the admin can use the app immediately
	_, err = s.repo.CompleteOnboarding(ctx, user.ID, "USD")
	if err != nil {
		return fmt.Errorf("completing admin onboarding: %w", err)
	}

	s.logger.Info("admin user seeded",
		slog.String("method", "SeedAdmin"),
		slog.String("user_id", user.ID),
		slog.String("username", username),
	)

	return nil
}

// ChangePassword validates the current password, checks the strength of the new
// password, hashes it, updates the user record, and revokes all existing tokens.
// Returns a fresh token pair so the current session stays active.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req *model.ChangePasswordRequest) (*model.User, *model.TokenPair, error) {
	// Look up the user to get their current password hash
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up user: %w", err)
	}
	if user == nil {
		return nil, nil, &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "User not found",
			Status:  401,
		}
	}

	// Verify current password
	if !s.password.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		return nil, nil, &AuthError{
			Code:    model.ErrInvalidCredentials,
			Message: "Current password is incorrect",
			Status:  401,
		}
	}

	// Validate new password strength
	if err := ValidatePasswordStrength(req.NewPassword); err != nil {
		return nil, nil, &AuthError{
			Code:    model.ErrWeakPassword,
			Message: err.Error(),
			Status:  400,
		}
	}

	// Hash new password
	hash, err := s.password.HashPassword(req.NewPassword)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing new password: %w", err)
	}

	// Update the password
	if err := s.repo.UpdatePassword(ctx, userID, hash); err != nil {
		return nil, nil, fmt.Errorf("updating password: %w", err)
	}

	// Revoke all existing tokens (forces re-login on other sessions)
	if err := s.repo.RevokeAllUserTokens(ctx, userID); err != nil {
		return nil, nil, fmt.Errorf("revoking tokens: %w", err)
	}

	// Generate fresh tokens for the current session
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Role, user.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	s.logger.Info("password changed",
		slog.String("method", "ChangePassword"),
		slog.String("user_id", user.ID),
	)

	return user, &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// protectedUsernames contains usernames that cannot be deleted.
var protectedUsernames = []string{"admin", "thompson"}

// isProtectedUsername returns true if the username is in the protected list.
func isProtectedUsername(username string) bool {
	for _, protected := range protectedUsernames {
		if username == protected {
			return true
		}
	}
	return false
}

// DeleteUser permanently deletes a user. Validates that the admin is not
// deleting themselves, that the target user is not protected, and that the
// admin's password is correct before performing the deletion.
func (s *AuthService) DeleteUser(ctx context.Context, adminUserID, targetUserID, password string) error {
	// Guard: cannot delete yourself
	if adminUserID == targetUserID {
		return &AuthError{
			Code:    model.ErrValidationError,
			Message: "Cannot delete your own account",
			Status:  400,
		}
	}

	// Look up admin to verify password
	adminUser, err := s.repo.GetUserByID(ctx, adminUserID)
	if err != nil {
		return fmt.Errorf("looking up admin user: %w", err)
	}
	if adminUser == nil {
		return &AuthError{
			Code:    model.ErrUnauthorized,
			Message: "Admin user not found",
			Status:  401,
		}
	}

	// Verify admin's password
	if !s.password.CheckPassword(password, adminUser.PasswordHash) {
		return &AuthError{
			Code:    model.ErrInvalidCredentials,
			Message: "Invalid password",
			Status:  401,
		}
	}

	// Look up target user
	targetUser, err := s.repo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("looking up target user: %w", err)
	}
	if targetUser == nil {
		return &AuthError{
			Code:    model.ErrNotFound,
			Message: "User not found",
			Status:  404,
		}
	}

	// Check if target user is protected
	if isProtectedUsername(targetUser.Username) {
		return &AuthError{
			Code:    model.ErrProtectedUser,
			Message: "Cannot delete a protected user",
			Status:  403,
		}
	}

	// Perform deletion
	if err := s.repo.DeleteUser(ctx, targetUserID); err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	s.logger.Info("user deleted",
		slog.String("method", "DeleteUser"),
		slog.String("admin_user_id", adminUserID),
		slog.String("deleted_user_id", targetUserID),
		slog.String("deleted_username", targetUser.Username),
	)

	return nil
}
