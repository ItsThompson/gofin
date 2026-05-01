package model

import "time"

// User represents a user in the auth domain.
type User struct {
	ID                     string    `json:"id"`
	Username               string    `json:"username"`
	Email                  string    `json:"email"`
	PasswordHash           string    `json:"-"`
	Role                   string    `json:"role"`
	Currency               string    `json:"currency"`
	HasCompletedOnboarding bool      `json:"hasCompletedOnboarding"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// UserResponse is the public-facing user representation (no password hash).
type UserResponse struct {
	ID                     string `json:"id"`
	Username               string `json:"username"`
	Email                  string `json:"email"`
	Role                   string `json:"role"`
	Currency               string `json:"currency"`
	HasCompletedOnboarding bool   `json:"hasCompletedOnboarding"`
	CreatedAt              string `json:"createdAt"`
}

// ToResponse converts a User to its public representation.
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:                     u.ID,
		Username:               u.Username,
		Email:                  u.Email,
		Role:                   u.Role,
		Currency:               u.Currency,
		HasCompletedOnboarding: u.HasCompletedOnboarding,
		CreatedAt:              u.CreatedAt.Format(time.RFC3339),
	}
}
