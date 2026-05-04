package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/model"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

// mockBlacklistRepository implements repository.BlacklistRepository for handler tests.
type mockBlacklistRepository struct {
	mock.Mock
}

func (m *mockBlacklistRepository) BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	args := m.Called(ctx, jti, userID, expiresAt)
	return args.Error(0)
}

func (m *mockBlacklistRepository) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	args := m.Called(ctx, jti)
	return args.Bool(0), args.Error(1)
}

func (m *mockBlacklistRepository) CleanupExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// mockUserRepository implements repository.UserRepository for handler tests.
type mockUserRepository struct {
	mock.Mock
}

func (m *mockUserRepository) CreateUser(ctx context.Context, username, email, passwordHash, role, currency string) (*model.User, error) {
	args := m.Called(ctx, username, email, passwordHash, role, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) CompleteOnboarding(ctx context.Context, userID string, currency string) (*model.User, error) {
	args := m.Called(ctx, userID, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) ListAllUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *mockUserRepository) UpdateUser(ctx context.Context, userID, username, email, currency string) (*model.User, error) {
	args := m.Called(ctx, userID, username, email, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockUserRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockUserRepository) GetTokensRevokedAt(ctx context.Context, userID string) (*time.Time, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func setupTestRouter(repo *mockUserRepository) *gin.Engine {
	return setupTestRouterWithBlacklist(repo, new(mockBlacklistRepository))
}

func setupTestRouterWithBlacklist(repo *mockUserRepository, blacklistRepo *mockBlacklistRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	handler := NewRESTHandler(authSvc, logger, false, "")
	r := gin.New()
	handler.RegisterRoutes(r)
	return r
}

func doJSON(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

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

	var errResp model.ApiError
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

	var errResp model.ApiError
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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
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

	var errResp model.ApiError
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

	var errResp model.ApiError
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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrInvalidCredentials, errResp.Code)
	assert.Equal(t, "Invalid email or password", errResp.Message)
}

// --- Helpers ---

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// doJSONWithCookies makes a JSON request with cookies attached.
func doJSONWithCookies(r *gin.Engine, method, path string, body interface{}, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

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

	blacklistRepo.On("IsTokenBlacklisted", mock.Anything, refreshClaims.ID).Return(false, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(&model.User{
		ID:       "user-123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
		Currency: "USD",
		CreatedAt: time.Now(),
	}, nil)
	blacklistRepo.On("BlacklistToken", mock.Anything, refreshClaims.ID, "user-123", mock.AnythingOfType("time.Time")).Return(nil)
	blacklistRepo.On("CleanupExpired", mock.Anything).Return(nil)

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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrUnauthorized, errResp.Code)
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

	blacklistRepo.On("IsTokenBlacklisted", mock.Anything, refreshClaims.ID).Return(true, nil)

	w := doJSONWithCookies(r, "POST", "/api/auth/refresh", nil, []*http.Cookie{
		{Name: "gofin_refresh", Value: refreshToken},
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrUnauthorized, errResp.Code)
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

// --- CompleteOnboarding Handler Tests ---

func doJSONWithUserID(r *gin.Engine, method, path, userID string, body interface{}) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrUnauthorized, errResp.Code)
}

func TestCompleteOnboardingHandler_InvalidBody(t *testing.T) {
	repo := new(mockUserRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/auth/onboarding-complete", "user-123", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
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

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
}
