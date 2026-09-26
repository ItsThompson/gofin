package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNoStorePersonalResponsesMarksPersonalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(NoStorePersonalResponses())
	engine.Any("/*path", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	for _, path := range []string{"/api/finance/summary", "/api/expenses", "/api/expenses/1"} {
		recording := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recording, request)

		assert.Equal(t, "no-store", recording.Header().Get("Cache-Control"), path)
	}
}

func TestNoStorePersonalResponsesLeavesOtherRoutesUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(NoStorePersonalResponses())
	engine.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	recording := httptest.NewRecorder()
	engine.ServeHTTP(recording, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Empty(t, recording.Header().Get("Cache-Control"))
}
