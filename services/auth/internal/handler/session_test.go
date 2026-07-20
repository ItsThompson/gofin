package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

// --- Refresh Handler Tests ---

func TestRefreshHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	r := setupTestRouterWithBlacklist(repo, blacklistRepo)

	// Generate a valid refresh token
	jwtSvc := service.NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("ConsumeToken", mock.Anything, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(true, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(&model.User{
		ID:        "user-123",
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      "user",
		Currency:  "USD",
		CreatedAt: time.Now(),
	}, nil)

	w := doJSONWithCookies(r, "POST", "/api/auth/refresh", nil, []*http.Cookie{
		{Name: "gofin_refresh", Value: refreshToken},
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.User.ID)

	// Verify new cookies are set
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie)
	assert.True(t, accessCookie.HttpOnly)
	assert.Equal(t, "/", accessCookie.Path)

	newRefreshCookie := findCookie(cookies, "gofin_refresh")
	require.NotNil(t, newRefreshCookie)
	assert.True(t, newRefreshCookie.HttpOnly)
	assert.Equal(t, "/api/auth", newRefreshCookie.Path)
	// The new refresh token should be different from the old one
	assert.NotEqual(t, refreshToken, newRefreshCookie.Value)
}

func TestRefreshHandler_NoCookie(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSON(r, "POST", "/api/auth/refresh", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeUnauthorized, errResp.Code)
}

func TestRefreshHandler_BlacklistedToken(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	r := setupTestRouterWithBlacklist(repo, blacklistRepo)

	jwtSvc := service.NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("ConsumeToken", mock.Anything, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(false, nil)

	w := doJSONWithCookies(r, "POST", "/api/auth/refresh", nil, []*http.Cookie{
		{Name: "gofin_refresh", Value: refreshToken},
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeUnauthorized, errResp.Code)
}

// --- Logout Handler Tests ---

func TestLogoutHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	blacklistRepo := new(mockBlacklistRepository)
	r := setupTestRouterWithBlacklist(repo, blacklistRepo)

	jwtSvc := service.NewJWTService("test-secret")
	_, refreshToken, err := jwtSvc.GenerateTokenPair("user-123", "user", "testuser")
	require.NoError(t, err)

	refreshClaims, err := jwtSvc.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	blacklistRepo.On("BlacklistToken", mock.Anything, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(nil)

	w := doJSONWithCookies(r, "POST", "/api/auth/logout", nil, []*http.Cookie{
		{Name: "gofin_refresh", Value: refreshToken},
	})

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify cookies are cleared
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie, "expected gofin_access cookie to be cleared")
	assert.Equal(t, -1, accessCookie.MaxAge)

	refreshClearCookie := findCookie(cookies, "gofin_refresh")
	require.NotNil(t, refreshClearCookie, "expected gofin_refresh cookie to be cleared")
	assert.Equal(t, -1, refreshClearCookie.MaxAge)
}

func TestLogoutHandler_NoCookie_StillClears(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// Logout without a refresh cookie should still return 204 and clear cookies
	w := doJSON(r, "POST", "/api/auth/logout", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)

	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie)
	assert.Equal(t, -1, accessCookie.MaxAge)
}
