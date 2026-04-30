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
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

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

func setupTestRouter(repo *mockUserRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, jwtSvc, pwdSvc, logger)

	handler := NewRESTHandler(authSvc, logger, false)
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
	assert.Equal(t, "/api", accessCookie.Path)

	refreshCookie := findCookie(cookies, "gofin_refresh")
	require.NotNil(t, refreshCookie, "expected gofin_refresh cookie")
	assert.True(t, refreshCookie.HttpOnly)
	assert.Equal(t, "/api/auth/refresh", refreshCookie.Path)
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
}

// --- Login Handler Tests ---

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
	assert.NotNil(t, findCookie(cookies, "gofin_access"))
	assert.NotNil(t, findCookie(cookies, "gofin_refresh"))
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
