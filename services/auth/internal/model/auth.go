package model

// RegisterRequest is the input for user registration.
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest is the input for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse is the JSON body returned on successful register/login.
// Tokens are sent via httpOnly cookies, not in the response body.
type AuthResponse struct {
	User *UserResponse `json:"user"`
}

// TokenPair holds the generated access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// ValidateTokenResult is the outcome of a successful token validation.
type ValidateTokenResult struct {
	UserID    string
	Role      string
	Username  string
	AssumedBy string
}

// CompleteOnboardingRequest is the input for marking onboarding as complete.
type CompleteOnboardingRequest struct {
	Currency string `json:"currency" binding:"required"`
}

// AssumeIdentityRequest is the input for admin identity assumption.
type AssumeIdentityRequest struct {
	UserID string `json:"userId" binding:"required"`
}

// UpdateProfileRequest is the input for PUT /api/auth/me (profile update).
type UpdateProfileRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Currency string `json:"currency" binding:"required"`
}

// ChangePasswordRequest is the input for POST /api/auth/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

// AdminUserResponse is the public-facing user representation for admin user lists.
type AdminUserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

// AdminUsersResponse wraps the list of users for the admin endpoint.
type AdminUsersResponse struct {
	Users []AdminUserResponse `json:"users"`
}
