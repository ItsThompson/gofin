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

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
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

func (m *mockExpenseRepository) GetExpenseByID(ctx context.Context, id string, userID string) (*model.Expense, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetExpenseByIdempotencyKey(ctx context.Context, userID string, key string) (*model.Expense, error) {
	// The key is now required, so every create performs a replay lookup. Most
	// create tests exercise the fresh-insert path and don't care about replays:
	// default to "no existing expense" unless a test registers an explicit
	// expectation for this method.
	for _, c := range m.ExpectedCalls {
		if c.Method == "GetExpenseByIdempotencyKey" {
			args := m.Called(ctx, userID, key)
			if args.Get(0) == nil {
				return nil, args.Error(1)
			}
			return args.Get(0).(*model.Expense), args.Error(1)
		}
	}
	return nil, nil
}

func (m *mockExpenseRepository) DeactivateExpense(ctx context.Context, id string, userID string) (int64, error) {
	args := m.Called(ctx, id, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseRepository) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	args := m.Called(ctx, userID, tagID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseRepository) CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, original, correction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, expenseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, groupID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetActiveExpenseSuggestionInputs(ctx context.Context, userID string) ([]*model.ExpenseSuggestionInput, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.ExpenseSuggestionInput), args.Error(1)
}

func (m *mockExpenseRepository) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockExpenseRepository) GetExpensesByUserAfter(ctx context.Context, userID string, cursor repository.ExpenseCursor, pageSize int32) ([]*model.Expense, repository.ExpenseCursor, bool, error) {
	args := m.Called(ctx, userID, cursor, pageSize)
	var rows []*model.Expense
	if args.Get(0) != nil {
		rows = args.Get(0).([]*model.Expense)
	}
	return rows, args.Get(1).(repository.ExpenseCursor), args.Bool(2), args.Error(3)
}

func setupTestRouter(repo *mockExpenseRepository) *gin.Engine {
	return setupTestRouterWithPeriod(repo, newTestPeriodClient())
}

// setupTestRouterWithPeriod builds a router wired to a custom period context
// client, so handler tests can exercise non-default reporting currencies.
func setupTestRouterWithPeriod(repo *mockExpenseRepository, periodClient *mockPeriodContextClient) *gin.Engine {
	return setupTestRouterWithFx(repo, periodClient, &stubFxClient{})
}

// setupTestRouterWithFx builds a router wired to a custom period context client
// and FX client, so handler tests can exercise foreign-currency conversion.
func setupTestRouterWithFx(repo *mockExpenseRepository, periodClient *mockPeriodContextClient, fxClient service.FxClient) *gin.Engine {
	return setupTestRouterWithFxClock(repo, periodClient, fxClient, time.Now)
}

// setupTestRouterWithFxClock builds a router with a fixed clock so FX tests can
// assert the exact request (including RequestedAt).
func setupTestRouterWithFxClock(repo *mockExpenseRepository, periodClient *mockPeriodContextClient, fxClient service.FxClient, clock func() time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, fxClient, clock, logger)
	h := NewRESTHandler(expenseSvc)
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
			ID:                                    "exp-123",
			UserID:                                "user-1",
			Name:                                  "Grocery shopping",
			ExpenseType:                           "essentials",
			TagID:                                 "tag-food",
			ExpenseDateIso:                        "2026-05-03",
			PeriodYear:                            2026,
			PeriodMonth:                           5,
			Status:                                "active",
			CreatedAt:                             "2026-05-03T10:00:00Z",
			OriginalTransactionAmountInMinorUnits: 2500,
		}, nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Grocery shopping",
		"amountInTransactionCurrencyMinorUnits": 2500,
		"expenseType":                           "essentials",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
		"clientGeneratedIdempotencyKey":         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-123", resp.Expense.ID)
	assert.Equal(t, "user-1", resp.Expense.UserID)
	assert.Equal(t, int64(2500), resp.Expense.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "essentials", resp.Expense.ExpenseType)
	assert.Equal(t, "active", resp.Expense.Status)
}

func TestCreateExpenseHandler_AcceptsTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)

	periodClient := new(mockPeriodContextClient)
	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "EUR",
	}, nil)

	repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(expense *model.Expense) bool {
		return expense.TransactionCurrencyCode == "EUR"
	})).Return(&model.Expense{
		ID:                                    "exp-123",
		UserID:                                "user-1",
		Name:                                  "Coffee",
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		Status:                                "active",
		CreatedAt:                             "2026-05-03T10:00:00Z",
		OriginalTransactionAmountInMinorUnits: 450,
		ReportingAmountInMinorUnits:           450,
		ReportingCurrencyCode:                 "EUR",
		SourceToTargetExchangeRate:            "1",
		ExchangeRateSource:                    exchangesource.Identity,
	}, nil)

	r := setupTestRouterWithPeriod(repo, periodClient)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Coffee",
		"amountInTransactionCurrencyMinorUnits": 450,
		"transactionCurrencyCode":               "EUR",
		"expenseType":                           "desires",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
		"clientGeneratedIdempotencyKey":         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "EUR", resp.Expense.TransactionCurrencyCode)
	// Canonical transaction and reporting money fields are present in the response.
	assert.Equal(t, int64(450), resp.Expense.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, int64(450), resp.Expense.ReportingAmountInMinorUnits)
	assert.Equal(t, "EUR", resp.Expense.ReportingCurrencyCode)
	assert.Equal(t, "1", resp.Expense.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.Identity, resp.Expense.ExchangeRateSource)
	repo.AssertExpectations(t)
}

// TestCreateExpenseHandler_ForeignCurrencyFxSuccess asserts a foreign-currency
// expense with a wired FX client returns 201 with both transaction and reporting
// money fields plus snapshot metadata.
func TestCreateExpenseHandler_ForeignCurrencyFxSuccess(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	requestedAt := now.UTC().Format(time.RFC3339)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req service.FxConvertRequest) bool {
		return req.Amount == 1250 &&
			req.SourceCurrency == "EUR" &&
			req.TargetCurrency == "USD" &&
			req.RequestedAt == requestedAt
	})).Return(&service.FxConvertResponse{
		ConvertedAmount: 1364,
		ExchangeRate:    "1.0912",
		RateTimestamp:   "2026-08-14T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-08-14T11:00:00Z",
	}, nil)

	repo.On("CreateExpense", mock.Anything, mock.Anything).Return(&model.Expense{
		ID:                                    "exp-fx-1",
		UserID:                                "user-1",
		Name:                                  "Cafe",
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		Status:                                "active",
		OriginalTransactionAmountInMinorUnits: 1250,
		ReportingAmountInMinorUnits:           1364,
		ReportingCurrencyCode:                 "USD",
		SourceToTargetExchangeRate:            "1.0912",
		ExchangeRateSource:                    exchangesource.OpenExchangeRates,
		ExchangeRateTimestamp:                 "2026-08-14T10:00:00Z",
		ExchangeRateCacheExpiresAt:            "2026-08-14T11:00:00Z",
	}, nil)

	r := setupTestRouterWithFxClock(repo, periodClient, fxClient, func() time.Time { return now })

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Cafe",
		"amountInTransactionCurrencyMinorUnits": 1250,
		"transactionCurrencyCode":               "EUR",
		"expenseType":                           "desires",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
		"clientGeneratedIdempotencyKey":         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(1250), resp.Expense.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "EUR", resp.Expense.TransactionCurrencyCode)
	assert.Equal(t, int64(1364), resp.Expense.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", resp.Expense.ReportingCurrencyCode)
	assert.Equal(t, "1.0912", resp.Expense.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.OpenExchangeRates, resp.Expense.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", resp.Expense.ExchangeRateTimestamp)
	assert.Equal(t, "2026-08-14T11:00:00Z", resp.Expense.ExchangeRateCacheExpiresAt)
}

// TestCreateExpenseHandler_ForeignCurrencyFxUnavailable asserts a foreign-currency
// expense with an FX failure returns 503 CONVERSION_UNAVAILABLE and does not write.
func TestCreateExpenseHandler_ForeignCurrencyFxUnavailable(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.Anything).Return(nil, &apierr.Error{
		Code:    model.ErrConversionUnavailable,
		Message: "currency conversion is unavailable",
		Status:  http.StatusServiceUnavailable,
	})

	r := setupTestRouterWithFx(repo, periodClient, fxClient)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Cafe",
		"amountInTransactionCurrencyMinorUnits": 1250,
		"transactionCurrencyCode":               "EUR",
		"expenseType":                           "desires",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
		"clientGeneratedIdempotencyKey":         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), model.ErrConversionUnavailable)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateExpenseHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "", map[string]interface{}{
		"name":                                  "Grocery shopping",
		"amountInTransactionCurrencyMinorUnits": 2500,
		"expenseType":                           "essentials",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
}

func TestCreateExpenseHandler_ValidationError(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	// Zero amount should fail validation
	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Coffee",
		"amountInTransactionCurrencyMinorUnits": 0,
		"expenseType":                           "desires",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Equal(t, "validation failed", errResp.Message)
	require.NotNil(t, errResp.Fields)
	assert.Equal(t, "amount must be positive", errResp.Fields["amountInTransactionCurrencyMinorUnits"])
}

// TestCreateExpenseHandler_MissingIdempotencyKey asserts a create without the
// required idempotency key is rejected with a 400 validation error before any
// ledger lookup or insert.
func TestCreateExpenseHandler_MissingIdempotencyKey(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Coffee",
		"amountInTransactionCurrencyMinorUnits": 2500,
		"expenseType":                           "essentials",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Equal(t, "clientGeneratedIdempotencyKey is required", errResp.Message)
	repo.AssertNotCalled(t, "GetExpenseByIdempotencyKey", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateExpenseHandler_InvalidExpenseType(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "Coffee",
		"amountInTransactionCurrencyMinorUnits": 500,
		"expenseType":                           "luxury",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.NotNil(t, errResp.Fields)
	assert.Contains(t, errResp.Fields["expenseType"], "essentials, desires, savings")
}

func TestCreateExpenseHandler_MultipleFieldErrors(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	// Missing name, zero amount, invalid type: should report all field errors
	w := doJSONWithUserID(r, "POST", "/api/expenses", "user-1", map[string]interface{}{
		"name":                                  "",
		"amountInTransactionCurrencyMinorUnits": 0,
		"expenseType":                           "invalid",
		"tagId":                                 "tag-food",
		"expenseDateIso":                        "2026-05-03",
		"periodYear":                            2026,
		"periodMonth":                           5,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.NotNil(t, errResp.Fields)
	assert.Len(t, errResp.Fields, 3) // name, amount, expenseType
	assert.NotEmpty(t, errResp.Fields["name"])
	assert.NotEmpty(t, errResp.Fields["amountInTransactionCurrencyMinorUnits"])
	assert.NotEmpty(t, errResp.Fields["expenseType"])
}

// --- GetExpenses Handler Tests ---

func TestGetExpensesHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries", Status: "active", ExpenseDateIso: "2026-05-02"},
		{ID: "exp-2", Name: "Coffee", Status: "active", ExpenseDateIso: "2026-05-01"},
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
		{ID: "exp-3", Name: "Lunch", Status: "active"},
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

	repo.On("GetExpenseByID", mock.Anything, "exp-123", "user-1").
		Return(&model.Expense{
			ID:          "exp-123",
			UserID:      "user-1",
			Name:        "Coffee",
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

	repo.On("GetExpenseByID", mock.Anything, "exp-999", "user-1").Return(nil, nil)

	r := setupTestRouter(repo)

	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-999", "user-1", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeNotFound, errResp.Code)
}

// --- CorrectExpense Handler Tests ---

// setupTestRouterWithClock configures the service with a fixed clock for correction tests.
func setupTestRouterWithClock(repo *mockExpenseRepository, now time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, func() time.Time { return now }, logger)
	h := NewRESTHandler(expenseSvc)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func TestCorrectExpenseHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	original := &model.Expense{
		ID: "exp-original", UserID: "user-1", Name: "Coffee",
		TransactionCurrencyCode: "USD", ExpenseType: "desires",
		TagID: "tag-food", ExpenseDateIso: "2026-05-01",
		PeriodYear: 2026, PeriodMonth: 5, Status: "active",
		CreatedAt:                             "2026-05-01T10:00:00Z",
		OriginalTransactionAmountInMinorUnits: 500,
		ReportingAmountInMinorUnits:           500,
		ReportingCurrencyCode:                 "USD",
		SourceToTargetExchangeRate:            "1",
		ExchangeRateSource:                    exchangesource.Identity,
		ExchangeRateTimestamp:                 "2026-05-01T10:00:00Z",
	}

	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)
	repo.On("CorrectExpense", mock.Anything, original, mock.AnythingOfType("*model.Expense")).
		Return(&model.Expense{
			ID: "exp-correction", UserID: "user-1", Name: "Updated Coffee",
			ExpenseType: "desires",
			TagID:       "tag-food", ExpenseDateIso: "2026-05-01",
			PeriodYear: 2026, PeriodMonth: 5, Status: "active",
			CorrectsID: "exp-original", CreatedAt: "2026-05-03T10:00:00Z",
		}, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "POST", "/api/expenses/exp-original/correct", "user-1", map[string]interface{}{
		"name": "Updated Coffee", "amountInTransactionCurrencyMinorUnits": 600,
		"expenseType": "desires", "tagId": "tag-food", "expenseDateIso": "2026-05-01",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp model.ExpenseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "exp-correction", resp.Expense.ID)
	assert.Equal(t, "exp-original", resp.Expense.CorrectsID)
	assert.Equal(t, "active", resp.Expense.Status)
}

func TestCorrectExpenseHandler_AlreadyCorrected(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	corrected := &model.Expense{
		ID: "exp-original", UserID: "user-1", Name: "Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food", ExpenseDateIso: "2026-05-01",
		PeriodYear: 2026, PeriodMonth: 5, Status: "corrected",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(corrected, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "POST", "/api/expenses/exp-original/correct", "user-1", map[string]interface{}{
		"name": "Updated Coffee", "amountInTransactionCurrencyMinorUnits": 600,
		"expenseType": "desires", "tagId": "tag-food", "expenseDateIso": "2026-05-01",
	})

	assert.Equal(t, http.StatusConflict, w.Code)
	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrAlreadyCorrected, errResp.Code)
}

func TestCorrectExpenseHandler_PeriodLocked(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	pastExpense := &model.Expense{
		ID: "exp-past", UserID: "user-1", Name: "Old Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food", ExpenseDateIso: "2026-04-15",
		PeriodYear: 2026, PeriodMonth: 4, Status: "active",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-past", "user-1").Return(pastExpense, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "POST", "/api/expenses/exp-past/correct", "user-1", map[string]interface{}{
		"name": "Updated", "amountInTransactionCurrencyMinorUnits": 600,
		"expenseType": "desires", "tagId": "tag-food", "expenseDateIso": "2026-04-15",
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodLocked, errResp.Code)
}

func TestCorrectExpenseHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "POST", "/api/expenses/exp-1/correct", "", map[string]interface{}{
		"name": "X", "amountInTransactionCurrencyMinorUnits": 100, "expenseType": "desires", "tagId": "t", "expenseDateIso": "2026-05-01",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCorrectExpenseHandler_InvalidBody(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)

	req := httptest.NewRequest("POST", "/api/expenses/exp-1/correct", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- GetCorrectionHistory Handler Tests ---

func TestGetCorrectionHistoryHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	chain := []*model.Expense{
		{ID: "exp-1", Name: "Original", Status: "corrected"},
		{ID: "exp-2", Name: "Correction", Status: "active", CorrectsID: "exp-1"},
	}
	repo.On("GetCorrectionHistory", mock.Anything, "exp-1", "user-1").Return(chain, nil)

	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-1/history", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.CorrectionHistoryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Entries, 2)
	assert.Equal(t, "exp-1", resp.Entries[0].ID)
	assert.Equal(t, "exp-2", resp.Entries[1].ID)
}

func TestGetCorrectionHistoryHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "GET", "/api/expenses/exp-1/history", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GetProRataGroup Handler Tests ---

func TestGetProRataGroupHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	expenses := []*model.Expense{
		{ID: "exp-1", ProRataIndex: 1, ProRataTotal: 3},
		{ID: "exp-2", ProRataIndex: 2, ProRataTotal: 3},
	}
	repo.On("GetProRataGroup", mock.Anything, "group-1", "user-1").Return(expenses, nil)

	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "GET", "/api/expenses/prorata/group-1", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp model.ProRataGroupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Expenses, 2)
}

func TestGetProRataGroupHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "GET", "/api/expenses/prorata/group-1", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- DeleteExpense Handler Tests ---

func TestDeleteExpenseHandler_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	original := &model.Expense{
		ID: "exp-original", UserID: "user-1", Name: "Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food", ExpenseDateIso: "2026-05-01",
		PeriodYear: 2026, PeriodMonth: 5, Status: "active",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)
	repo.On("DeactivateExpense", mock.Anything, "exp-original", "user-1").Return(int64(1), nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "DELETE", "/api/expenses/exp-original", "user-1", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
	repo.AssertExpectations(t)
}

func TestDeleteExpenseHandler_AlreadyCorrected(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	corrected := &model.Expense{
		ID: "exp-original", UserID: "user-1", Name: "Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food", ExpenseDateIso: "2026-05-01",
		PeriodYear: 2026, PeriodMonth: 5, Status: "corrected",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(corrected, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "DELETE", "/api/expenses/exp-original", "user-1", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrAlreadyCorrected, errResp.Code)
}

func TestDeleteExpenseHandler_PeriodLocked(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	pastExpense := &model.Expense{
		ID: "exp-past", UserID: "user-1", Name: "Old Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food", ExpenseDateIso: "2026-04-15",
		PeriodYear: 2026, PeriodMonth: 4, Status: "active",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-past", "user-1").Return(pastExpense, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "DELETE", "/api/expenses/exp-past", "user-1", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodLocked, errResp.Code)
}

func TestDeleteExpenseHandler_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	repo.On("GetExpenseByID", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	r := setupTestRouterWithClock(repo, now)
	w := doJSONWithUserID(r, "DELETE", "/api/expenses/exp-missing", "user-1", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeNotFound, errResp.Code)
}

func TestDeleteExpenseHandler_MissingUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	r := setupTestRouter(repo)
	w := doJSONWithUserID(r, "DELETE", "/api/expenses/exp-1", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
