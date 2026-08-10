package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

func setupTestRouter(repo *mockFinanceRepository, txBeginner *mockTxBeginner) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)

	h := NewRESTHandler(financeSvc)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func setupTestRouterWithExpenseClient(repo *mockFinanceRepository, txBeginner *mockTxBeginner, expClient *mockExpenseClient) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, expClient, time.Now, logger)

	h := NewRESTHandler(financeSvc)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func setupTestRouterWithNowFunc(repo *mockFinanceRepository, txBeginner *mockTxBeginner, expClient *mockExpenseClient, nowFunc func() time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, expClient, nowFunc, logger)

	h := NewRESTHandler(financeSvc)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
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
