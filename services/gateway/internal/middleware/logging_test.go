package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// logEntry represents a parsed structured log line.
type logEntry struct {
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	UserID     string `json:"user_id,omitempty"`
	ClientIP   string `json:"client_ip"`
}

func TestRequestLogger_LogsRequestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "GET", entry.Method)
	assert.Equal(t, "/api/test", entry.Path)
	assert.Equal(t, http.StatusOK, entry.Status)
	assert.GreaterOrEqual(t, entry.DurationMs, int64(0))
	assert.Equal(t, "INFO", entry.Level)
}

func TestRequestLogger_IncludesUserID_WhenAuthenticated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-User-ID", "user-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "user-123", entry.UserID)
}

func TestRequestLogger_OmitsUserID_WhenUnauthenticated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Empty(t, entry.UserID)
}

func TestRequestLogger_LogsWarnFor4xx(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "WARN", entry.Level)
	assert.Equal(t, http.StatusNotFound, entry.Status)
}

func TestRequestLogger_LogsErrorFor5xx(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "ERROR", entry.Level)
	assert.Equal(t, http.StatusInternalServerError, entry.Status)
}
