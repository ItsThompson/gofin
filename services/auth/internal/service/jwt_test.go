package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTokenPair(t *testing.T) {
	jwt := NewJWTService("test-secret")

	access, refresh, err := jwt.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	assert.NotEqual(t, access, refresh)
}

func TestValidateAccessToken_Valid(t *testing.T) {
	jwt := NewJWTService("test-secret")

	access, _, err := jwt.GenerateTokenPair("user-123", "admin", "janedoe")
	require.NoError(t, err)

	claims, err := jwt.ValidateAccessToken(access)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "janedoe", claims.Username)
	assert.Empty(t, claims.AssumedBy)
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	jwtSigner := NewJWTService("secret-a")
	jwtValidator := NewJWTService("secret-b")

	access, _, err := jwtSigner.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)

	_, err = jwtValidator.ValidateAccessToken(access)
	assert.Error(t, err)
}

func TestValidateAccessToken_Garbage(t *testing.T) {
	jwt := NewJWTService("test-secret")

	_, err := jwt.ValidateAccessToken("not-a-token")
	assert.Error(t, err)
}

func TestValidateAccessToken_EmptyString(t *testing.T) {
	jwt := NewJWTService("test-secret")

	_, err := jwt.ValidateAccessToken("")
	assert.Error(t, err)
}

func TestRefreshTokenContainsJTI(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")

	_, refresh1, err := jwtSvc.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)

	_, refresh2, err := jwtSvc.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)

	// Each refresh token should have a unique JTI
	assert.NotEqual(t, refresh1, refresh2)
}
