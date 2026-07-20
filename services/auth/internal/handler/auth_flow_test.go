package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

// --- Register Handler Tests ---

func TestRegisterHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", mock.Anything, "testuser").Return(nil, nil)
	repo.On("CreateUser", mock.Anything, "testuser", "test@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(&model.User{
			ID:        "user-123",
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "user",
			Currency:  "USD",
			CreatedAt: time.Now(),
		}, nil)

	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.User.ID)
	assert.Equal(t, "testuser", resp.User.Username)

	// Verify httpOnly cookies are set
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie, "expected gofin_access cookie")
	assert.True(t, accessCookie.HttpOnly)
	assert.Equal(t, "/", accessCookie.Path)
	assert.Equal(t, http.SameSiteStrictMode, accessCookie.SameSite)
	assert.Equal(t, 900, accessCookie.MaxAge) // 15 minutes

	refreshCookie := findCookie(cookies, "gofin_refresh")
	require.NotNil(t, refreshCookie, "expected gofin_refresh cookie")
	assert.True(t, refreshCookie.HttpOnly)
	assert.Equal(t, "/api/auth", refreshCookie.Path)
	assert.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
	assert.Equal(t, 604800, refreshCookie.MaxAge) // 7 days
}

func TestRegisterHandler_WeakPassword(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "weak",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrWeakPassword, errResp.Code)
}

func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByEmail", mock.Anything, "taken@example.com").Return(&model.User{
		ID:    "existing",
		Email: "taken@example.com",
	}, nil)

	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "newuser",
		"email":    "taken@example.com",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDuplicateEmail, errResp.Code)
}

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	// Malformed JSON carries no per-field detail, so fields is omitted.
	assert.Nil(t, errResp.Fields)
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// Missing password
	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "testuser",
		"email":    "test@example.com",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
}

// TestRegisterHandler_ValidationFields_C6 asserts that a validator-detected
// field error surfaces the offending field in the response `fields` map.
func TestRegisterHandler_ValidationFields_C6(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// Valid username + password but a malformed email fails the `email` rule.
	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "testuser",
		"email":    "not-an-email",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	require.NotEmpty(t, errResp.Fields, "validation error should carry field detail")
	assert.Contains(t, errResp.Fields, "Email")
}

// --- Login Handler Tests ---

func TestRegisterHandler_DuplicateEmailFromConstraint(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// Both uniqueness checks pass (TOCTOU race), but INSERT hits constraint
	repo.On("GetUserByEmail", mock.Anything, "race@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", mock.Anything, "raceuser").Return(nil, nil)
	repo.On("CreateUser", mock.Anything, "raceuser", "race@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(nil, &repository.DuplicateError{Constraint: "users_email_key"})

	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "raceuser",
		"email":    "race@example.com",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDuplicateEmail, errResp.Code)
}

func TestRegisterHandler_DuplicateUsernameFromConstraint(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByEmail", mock.Anything, "unique@example.com").Return(nil, nil)
	repo.On("GetUserByUsername", mock.Anything, "takenuser").Return(nil, nil)
	repo.On("CreateUser", mock.Anything, "takenuser", "unique@example.com", mock.AnythingOfType("string"), "user", "USD").
		Return(nil, &repository.DuplicateError{Constraint: "users_username_key"})

	w := doJSON(r, "POST", "/api/auth/register", map[string]string{
		"username": "takenuser",
		"email":    "unique@example.com",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDuplicateUsername, errResp.Code)
}

func TestLoginHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	pwdSvc := service.NewPasswordService(4)
	hash, _ := pwdSvc.HashPassword("ValidPass1")

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(&model.User{
		ID:           "user-123",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hash,
		Role:         "user",
		Currency:     "USD",
		CreatedAt:    time.Now(),
	}, nil)

	w := doJSON(r, "POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "ValidPass1",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.User.ID)

	// Verify cookies
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie, "expected gofin_access cookie")
	assert.True(t, accessCookie.HttpOnly)
	assert.Equal(t, "/", accessCookie.Path)
	assert.Equal(t, http.SameSiteStrictMode, accessCookie.SameSite)
	assert.Equal(t, 900, accessCookie.MaxAge)

	refreshCookie := findCookie(cookies, "gofin_refresh")
	require.NotNil(t, refreshCookie, "expected gofin_refresh cookie")
	assert.True(t, refreshCookie.HttpOnly)
	assert.Equal(t, "/api/auth", refreshCookie.Path)
	assert.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
	assert.Equal(t, 604800, refreshCookie.MaxAge)
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, nil)

	w := doJSON(r, "POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "AnyPass123",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrInvalidCredentials, errResp.Code)
	assert.Equal(t, "Invalid email or password", errResp.Message)
}
