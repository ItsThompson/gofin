package service

import (
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService handles password hashing and validation.
type PasswordService struct {
	bcryptCost int
}

// NewPasswordService creates a new PasswordService with the given bcrypt cost.
func NewPasswordService(bcryptCost int) *PasswordService {
	return &PasswordService{bcryptCost: bcryptCost}
}

// HashPassword hashes a plaintext password using bcrypt.
func (p *PasswordService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), p.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func (p *PasswordService) CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ValidatePasswordStrength checks the password meets the strength requirements:
// 8+ chars, at least 1 uppercase, 1 lowercase, 1 digit.
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit")
	}

	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("password must be at least 8 characters with one uppercase letter, one lowercase letter, and one digit")
	}

	return nil
}
