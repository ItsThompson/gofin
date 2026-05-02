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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
)

// mockExpenseRepository implements repository.ExpenseRepository for handler tests.
type mockExpenseRepository struct {
	mock.Mock
}

func (m *mockExpenseRepository) CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, expense)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error) {
	args := m.Called(ctx, userID, year, month, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*model.Expense), args.Get(1).(int64), args.Error(2)
}

func (m *mockExpenseRepository) GetExpenseByID(ctx context.Context, id string) (*model.Expense, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func setupTestRouter(repo *mockExpenseRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, logger)
	h := NewRESTHandler(expenseSvc, logger)
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

// --- CreateExpense Handler Tests ---

func TestCreateExpenseHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)

	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Return(&model.Expense{
			ID:          "exp-123",
			UserID:      "user-1",
			Name:        "Grocery shopping",
			Amount:      2500,
			Currency:    "USD",
			ExpenseType: "essentials",
			TagID:       "tag-food",
			ExpenseDate: "2026-05-03",
			PeriodYear:  2026,
			PeriodMonth: 5,
			Status:      "active",
			CreatedAt:   "2026-05-03T10:00:00Z",
		}, nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":        "Grocery shopping",
		"amount":      2500,
		"currency":    "USD",
		"expenseType": "essentials",
		"tagId":       "tag-food",
		"expenseDate": "2026-05-03",
		"periodYear":  2026,
		"periodMonth": 5,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-123", resp.Expense.ID)
	assert.Equal(t, "user-1", resp.Expense.UserID)
	assert.Equal(t, int64(2500), resp.Expense.Amount)
	assert.Equal(t, "essentials", resp.Expense.ExpenseType)
	assert.Equal(t, "active", resp.Expense.Status)
}

func TestCreateExpenseHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "", map[string]interface{}{
		"name":        "Grocery shopping",
		"amount":      2500,
		"currency":    "USD",
		"expenseType": "essentials",
		"tagId":       "tag-food",
		"expenseDate": "2026-05-03",
		"periodYear":  2026,
		"periodMonth": 5,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateExpenseHandler_InvalidBody(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	req := httptest.NewRequest("POST", "/api/expenses", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
}

func TestCreateExpenseHandler_ValidationError(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	// Zero amount should fail validation
	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":        "Coffee",
		"amount":      0,
		"currency":    "USD",
		"expenseType": "desires",
		"tagId":       "tag-food",
		"expenseDate": "2026-05-03",
		"periodYear":  2026,
		"periodMonth": 5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
}

func TestCreateExpenseHandler_InvalidExpenseType(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":        "Coffee",
		"amount":      500,
		"currency":    "USD",
		"expenseType": "luxury",
		"tagId":       "tag-food",
		"expenseDate": "2026-05-03",
		"periodYear":  2026,
		"periodMonth": 5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- GetExpenses Handler Tests ---

func TestGetExpensesHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries", Amount: 5000, Status: "active", ExpenseDate: "2026-05-02"},
		{ID: "exp-2", Name: "Coffee", Amount: 500, Status: "active", ExpenseDate: "2026-05-01"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return(expenses, int64(2), nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses?year=2026&month=5", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ExpenseListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, int64(2), resp.Total)
	assert.Equal(t, int32(1), resp.Page)
	assert.Equal(t, int32(50), resp.PageSize)
	assert.False(t, resp.HasMore)
}

func TestGetExpensesHandler_WithPagination(t *testing.T) {
	repo := new(mockExpenseRepository)

	expenses := []*model.Expense{
		{ID: "exp-3", Name: "Lunch", Amount: 1500, Status: "active"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(2), int32(10)).
		Return(expenses, int64(15), nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses?year=2026&month=5&page=2&pageSize=10", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ExpenseListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, int64(15), resp.Total)
	assert.Equal(t, int32(2), resp.Page)
	assert.Equal(t, int32(10), resp.PageSize)
	assert.False(t, resp.HasMore) // page 2 * pageSize 10 = 20 >= 15
}

func TestGetExpensesHandler_MissingQueryParams(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", "/api/expenses"},
		{"missing month", "/api/expenses?year=2026"},
		{"missing year", "/api/expenses?month=5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSONWithUserID(r, "GET", tt.query, "user-1", nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetExpensesHandler_InvalidQueryParams(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses?year=abc&month=5", "user-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSONWithUserID(r, "GET", "/api/expenses?year=2026&month=abc", "user-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetExpensesHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses?year=2026&month=5", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GetExpense Handler Tests ---

func TestGetExpenseHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)

	repo.On("GetExpenseByID", mock.Anything, "exp-123").
		Return(&model.Expense{
			ID:          "exp-123",
			UserID:      "user-1",
			Name:        "Coffee",
			Amount:      500,
			Currency:    "USD",
			ExpenseType: "desires",
			Status:      "active",
		}, nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-123", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-123", resp.Expense.ID)
	assert.Equal(t, "Coffee", resp.Expense.Name)
}

func TestGetExpenseHandler_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)

	repo.On("GetExpenseByID", mock.Anything, "exp-999").Return(nil, nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-999", "user-1", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrNotFound, errResp.Code)
}

func TestGetExpenseHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-123", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
