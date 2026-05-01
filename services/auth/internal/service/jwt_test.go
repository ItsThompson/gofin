package service

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
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

func TestValidateAccessToken_Expired(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")

	// Construct a token that expired 1 hour ago
	now := time.Now()
	claims := AccessTokenClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-123",
			IssuedAt:  gojwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: gojwt.NewNumericDate(now.Add(-1 * time.Hour)),
		},
		Role:     "user",
		Username: "johndoe",
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	_, err = jwtSvc.ValidateAccessToken(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// --- Refresh Token Validation ---

func TestValidateRefreshToken_Valid(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")

	_, refresh, err := jwtSvc.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)

	claims, err := jwtSvc.ValidateRefreshToken(refresh)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.NotEmpty(t, claims.ID, "refresh token must have a JTI")
}

func TestValidateRefreshToken_WrongSecret(t *testing.T) {
	signer := NewJWTService("secret-a")
	validator := NewJWTService("secret-b")

	_, refresh, err := signer.GenerateTokenPair("user-123", "user", "johndoe")
	require.NoError(t, err)

	_, err = validator.ValidateRefreshToken(refresh)
	assert.Error(t, err)
}

func TestValidateRefreshToken_Expired(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")

	now := time.Now()
	claims := RefreshTokenClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-123",
			ID:        "some-jti",
			IssuedAt:  gojwt.NewNumericDate(now.Add(-8 * 24 * time.Hour)),
			ExpiresAt: gojwt.NewNumericDate(now.Add(-1 * 24 * time.Hour)),
		},
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	_, err = jwtSvc.ValidateRefreshToken(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateRefreshToken_Garbage(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")

	_, err := jwtSvc.ValidateRefreshToken("not-a-token")
	assert.Error(t, err)
}

func TestRefreshTokenTTL(t *testing.T) {
	jwtSvc := NewJWTService("test-secret")
	assert.Equal(t, 7*24*time.Hour, jwtSvc.RefreshTokenTTL())
}
