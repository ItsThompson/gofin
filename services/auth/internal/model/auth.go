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
