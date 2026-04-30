package service

import (
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
