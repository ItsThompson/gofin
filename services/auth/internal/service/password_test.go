package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePasswordStrength_Valid(t *testing.T) {
	valid := []string{
		"Password1",
		"Abcdefg1",
		"MyP4ssword",
		"StrongP1",
		"aB3defgh",
	}
	for _, pw := range valid {
		assert.NoError(t, ValidatePasswordStrength(pw), "expected %q to be valid", pw)
	}
}

func TestValidatePasswordStrength_MaxLength_Exactly72(t *testing.T) {
	// 72 bytes exactly: should pass (boundary)
	// "Ab1" + 69 'x' chars = 72 bytes total
	pw := "Ab1" + strings.Repeat("x", 69)
	assert.Equal(t, 72, len(pw))
	assert.NoError(t, ValidatePasswordStrength(pw))
}

func TestValidatePasswordStrength_MaxLength_73Rejected(t *testing.T) {
	// 73 bytes: should be rejected
	pw := "Ab1" + strings.Repeat("x", 70)
	assert.Equal(t, 73, len(pw))
	err := ValidatePasswordStrength(pw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password must not exceed 72 characters")
}

func TestValidatePasswordStrength_MaxLength_71Accepted(t *testing.T) {
	// 71 bytes with valid complexity: should pass
	pw := "Ab1" + strings.Repeat("x", 68)
	assert.Equal(t, 71, len(pw))
	assert.NoError(t, ValidatePasswordStrength(pw))
}

func TestValidatePasswordStrength_MaxLength_MultiByte(t *testing.T) {
	// Multi-byte UTF-8 characters: len() returns byte count.
	// 'é' is 2 bytes in UTF-8. A password of 37 'é' chars = 74 bytes > 72.
	// Even though it's only 37 characters, it should be rejected because
	// bcrypt uses byte length.
	pw := "A1" + strings.Repeat("é", 36) // 2 + 72 = 74 bytes
	assert.True(t, len(pw) > 72, "expected >72 bytes, got %d", len(pw))
	err := ValidatePasswordStrength(pw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password must not exceed 72 characters")

	// Conversely, 35 'é' chars + "A1" = 2 + 70 = 72 bytes: should pass
	pw2 := "A1" + strings.Repeat("é", 35)
	assert.Equal(t, 72, len(pw2))
	assert.NoError(t, ValidatePasswordStrength(pw2))
}

func TestValidatePasswordStrength_MaxLength_VeryLong(t *testing.T) {
	// A very long password (1000 bytes) should be rejected
	pw := "Ab1" + strings.Repeat("x", 997)
	assert.Equal(t, 1000, len(pw))
	err := ValidatePasswordStrength(pw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password must not exceed 72 characters")
}

func TestValidatePasswordStrength_TooShort(t *testing.T) {
	err := ValidatePasswordStrength("Abcde1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 8 characters")
}

func TestValidatePasswordStrength_NoUppercase(t *testing.T) {
	err := ValidatePasswordStrength("lowercase1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one uppercase letter")
}

func TestValidatePasswordStrength_NoLowercase(t *testing.T) {
	err := ValidatePasswordStrength("UPPERCASE1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one lowercase letter")
}

func TestValidatePasswordStrength_NoDigit(t *testing.T) {
	err := ValidatePasswordStrength("NoDigitHere")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one digit")
}

func TestValidatePasswordStrength_Empty(t *testing.T) {
	err := ValidatePasswordStrength("")
	assert.Error(t, err)
}

func TestHashAndCheckPassword(t *testing.T) {
	// Use cost 4 for fast tests (minimum bcrypt allows)
	svc := NewPasswordService(4)

	hash, err := svc.HashPassword("TestPassword1")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "TestPassword1", hash)

	assert.True(t, svc.CheckPassword("TestPassword1", hash))
	assert.False(t, svc.CheckPassword("WrongPassword1", hash))
}
