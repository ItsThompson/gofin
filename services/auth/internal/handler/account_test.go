package handler

import (
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
)

// --- CompleteOnboarding Handler Tests ---

func TestCompleteOnboardingHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("CompleteOnboarding", mock.Anything, "user-123", "EUR").
		Return(&model.User{
			ID:                     "user-123",
			Username:               "testuser",
			Email:                  "test@example.com",
			Role:                   "user",
			Currency:               "EUR",
			HasCompletedOnboarding: true,
			CreatedAt:              time.Now(),
		}, nil)

	w := doJSONWithUserID(r, "POST", "/api/auth/onboarding-complete", "user-123", map[string]string{
		"currency": "EUR",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.User.ID)
	assert.Equal(t, "EUR", resp.User.Currency)
	assert.True(t, resp.User.HasCompletedOnboarding)
}

func TestCompleteOnboardingHandler_MissingUserID(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/auth/onboarding-complete", "", map[string]string{
		"currency": "EUR",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeUnauthorized, errResp.Code)
}

func TestCompleteOnboardingHandler_InvalidBody(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/auth/onboarding-complete", "user-123", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
}

// --- ListUsers Handler Tests ---

func TestListUsersHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("ListAllUsers", mock.Anything).Return([]*model.User{
		{ID: "user-1", Username: "alice", Email: "alice@example.com", Role: "user", CreatedAt: time.Now()},
		{ID: "admin-1", Username: "admin", Email: "admin@example.com", Role: "admin", CreatedAt: time.Now()},
	}, nil)

	w := doJSON(r, "GET", "/api/admin/users", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AdminUsersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Users, 2)
	assert.Equal(t, "alice", resp.Users[0].Username)
	assert.Equal(t, "admin", resp.Users[1].Username)
	assert.Equal(t, "admin", resp.Users[1].Role)
}

// --- AssumeIdentity Handler Tests ---

func TestAssumeIdentityHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByID", mock.Anything, "target-456").Return(&model.User{
		ID:        "target-456",
		Username:  "targetuser",
		Email:     "target@example.com",
		Role:      "user",
		CreatedAt: time.Now(),
	}, nil)

	w := doJSONWithUserID(r, "POST", "/api/auth/assume", "admin-123", map[string]string{
		"userId": "target-456",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "target-456", resp.User.ID)
	assert.Equal(t, "targetuser", resp.User.Username)

	// Verify cookies are set
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie)
	assert.True(t, accessCookie.HttpOnly)
}

func TestAssumeIdentityHandler_MissingUserID(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// No X-User-ID header
	w := doJSON(r, "POST", "/api/auth/assume", map[string]string{
		"userId": "target-456",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAssumeIdentityHandler_InvalidBody(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/auth/assume", "admin-123", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssumeIdentityHandler_SelfAssumption(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/auth/assume", "admin-123", map[string]string{
		"userId": "admin-123",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- RestoreIdentity Handler Tests ---

func TestRestoreIdentityHandler_Success(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	repo.On("GetUserByID", mock.Anything, "admin-123").Return(&model.User{
		ID:        "admin-123",
		Username:  "admin",
		Email:     "admin@example.com",
		Role:      "admin",
		CreatedAt: time.Now(),
	}, nil)

	// Simulate a request with the X-Assumed-By header (set by gateway)
	req := httptest.NewRequest("POST", "/api/auth/restore", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Assumed-By", "admin-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "admin-123", resp.User.ID)
	assert.Equal(t, "admin", resp.User.Username)

	// Verify cookies are set
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "gofin_access")
	require.NotNil(t, accessCookie)
}

func TestRestoreIdentityHandler_NoAssumedBy(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	// No X-Assumed-By header
	w := doJSON(r, "POST", "/api/auth/restore", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
}
