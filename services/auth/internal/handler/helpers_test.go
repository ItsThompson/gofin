package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

func setupTestRouter(repo *mockUserRepository) *gin.Engine {
	return setupTestRouterWithBlacklist(repo, new(mockBlacklistRepository))
}

func setupTestRouterWithBlacklist(repo *mockUserRepository, blacklistRepo *mockBlacklistRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(repo, blacklistRepo, jwtSvc, pwdSvc, logger)

	// TTLs match the JWTService defaults so cookie max-ages stay 900 / 604800.
	handler := NewRESTHandler(authSvc, logger, false, "", service.DefaultAccessTokenTTL, service.DefaultRefreshTokenTTL)
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
