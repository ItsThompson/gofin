package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

// mockFinanceRepository implements repository.FinanceRepository for handler tests.
type mockFinanceRepository struct {
	mock.Mock
}

func (m *mockFinanceRepository) UpsertDefaults(ctx context.Context, settings *model.DefaultSettings) (*model.DefaultSettings, error) {
	args := m.Called(ctx, settings)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DefaultSettings), args.Error(1)
}

func (m *mockFinanceRepository) GetDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DefaultSettings), args.Error(1)
}

func (m *mockFinanceRepository) CreateTag(ctx context.Context, userID, name string, isDefault bool) (*model.Tag, error) {
	args := m.Called(ctx, userID, name, isDefault)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockFinanceRepository) ListTags(ctx context.Context, userID string) ([]*model.Tag, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Tag), args.Error(1)
}

func (m *mockFinanceRepository) CountUserTags(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockFinanceRepository) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockFinanceRepository) CreatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

// mockTxBeginner implements repository.TxBeginner.
type mockTxBeginner struct {
	mock.Mock
}

func (m *mockTxBeginner) BeginTx(ctx context.Context) (repository.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.Tx), args.Error(1)
}

// mockTx implements repository.Tx.
type mockTx struct {
	mock.Mock
	repo repository.FinanceRepository
}

func (m *mockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTx) Repo() repository.FinanceRepository {
	return m.repo
}

func setupTestRouter(repo *mockFinanceRepository, txBeginner *mockTxBeginner) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, logger)

	h := NewRESTHandler(financeSvc, logger)
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

// --- CompleteOnboarding Handler Tests ---

func TestCompleteOnboardingHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	txRepo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "USD",
		}, nil)

	// 10 default tags
	for _, tagName := range service.DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-123", tagName, true).
			Return(&model.Tag{ID: "tag-" + tagName, Name: tagName, IsDefault: true}, nil)
	}

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.DefaultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.Defaults.UserID)
	assert.Equal(t, int64(300000), resp.Defaults.BudgetAmount)
	assert.Equal(t, int32(50), resp.Defaults.EssentialsPercent)
	assert.Equal(t, "USD", resp.Defaults.Currency)

	tx.AssertCalled(t, "Commit", mock.Anything)
}

func TestCompleteOnboardingHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)

	r := setupTestRouter(repo, txBeginner)

	// Split sums to 99, not 100
	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestCompleteOnboardingHandler_ZeroPercentAllocations(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	// 100/0/0 is a valid split: user puts everything in essentials
	txRepo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      500000,
			EssentialsPercent: 100,
			DesiresPercent:    0,
			SavingsPercent:    0,
			Currency:          "USD",
		}, nil)

	for _, tagName := range service.DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-123", tagName, true).
			Return(&model.Tag{ID: "tag-" + tagName, Name: tagName, IsDefault: true}, nil)
	}

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 100,
		"desiresPercent":    0,
		"savingsPercent":    0,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.DefaultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int32(100), resp.Defaults.EssentialsPercent)
	assert.Equal(t, int32(0), resp.Defaults.DesiresPercent)
	assert.Equal(t, int32(0), resp.Defaults.SavingsPercent)
}

func TestCompleteOnboardingHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCompleteOnboardingHandler_TagSeedingFailureRollsBack(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)

	txRepo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "USD",
		}, nil)

	// First 3 tags succeed, 4th fails
	for i, tagName := range service.DefaultTags {
		if i < 3 {
			txRepo.On("CreateTag", mock.Anything, "user-123", tagName, true).
				Return(&model.Tag{ID: "tag-" + tagName, Name: tagName, IsDefault: true}, nil)
		} else if i == 3 {
			txRepo.On("CreateTag", mock.Anything, "user-123", tagName, true).
				Return(nil, fmt.Errorf("unique constraint violation"))
		}
		// Tags after index 3 are never reached
	}

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrInternalServerError, errResp.Code)

	// Verify rollback was called (commit should NOT have been called)
	tx.AssertCalled(t, "Rollback", mock.Anything)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
}

func TestCompleteOnboardingHandler_SkipDefaults(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	// Default skip values: USD, $0 budget, 50/30/20
	txRepo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      0,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "USD",
		}, nil)

	for _, tagName := range service.DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-123", tagName, true).
			Return(&model.Tag{ID: "tag-" + tagName, Name: tagName, IsDefault: true}, nil)
	}

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      0,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.DefaultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Defaults.BudgetAmount)
}

// --- GetDefaults Handler Tests ---

func TestGetDefaultsHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetDefaults", mock.Anything, "user-123").
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "USD",
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/defaults", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.DefaultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.Defaults.UserID)
}

func TestGetDefaultsHandler_NotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetDefaults", mock.Anything, "user-999").Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/defaults", "user-999", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrNotFound, errResp.Code)
}

// --- GetCurrentPeriod Handler Tests ---

func TestGetCurrentPeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-abc", resp.Period.ID)
	assert.Equal(t, int32(2026), resp.Period.Year)
	assert.Equal(t, int32(5), resp.Period.Month)
	assert.Equal(t, int64(300000), resp.Period.BudgetAmount)
}

func TestGetCurrentPeriodHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
	assert.Contains(t, errResp.Message, "2026-05")
}

func TestGetCurrentPeriodHandler_MissingQueryParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", "/api/finance/periods/current"},
		{"missing month", "/api/finance/periods/current?year=2026"},
		{"missing year", "/api/finance/periods/current?month=5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSONWithUserID(r, "GET", tt.query, "user-123", nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetCurrentPeriodHandler_InvalidQueryParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=abc&month=5", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=abc", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCurrentPeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/periods/current?year=2026&month=5", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- CreatePeriod Handler Tests ---

func TestCreatePeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(&model.BudgetPeriod{
			ID:                "period-new",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-new", resp.Period.ID)
	assert.Equal(t, int32(2026), resp.Period.Year)
	assert.Equal(t, int32(5), resp.Period.Month)
	assert.Equal(t, int64(300000), resp.Period.BudgetAmount)
}

func TestCreatePeriodHandler_ZeroBudget(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).
		Return(&model.BudgetPeriod{
			ID:                "period-zero",
			UserID:            "user-123",
			Year:              2026,
			Month:             5,
			BudgetAmount:      0,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      0,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Period.BudgetAmount)
}

func TestCreatePeriodHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestCreatePeriodHandler_InvalidMonth(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "user-123", map[string]interface{}{
		"year":              2026,
		"month":             13,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/periods", "", map[string]interface{}{
		"year":              2026,
		"month":             5,
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- UpdateDefaults Handler Tests ---

func TestUpdateDefaultsHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      500000,
			EssentialsPercent: 60,
			DesiresPercent:    20,
			SavingsPercent:    20,
			Currency:          "EUR",
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/defaults", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 60,
		"desiresPercent":    20,
		"savingsPercent":    20,
		"currency":          "EUR",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.DefaultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-123", resp.Defaults.UserID)
	assert.Equal(t, int64(500000), resp.Defaults.BudgetAmount)
	assert.Equal(t, int32(60), resp.Defaults.EssentialsPercent)
	assert.Equal(t, "EUR", resp.Defaults.Currency)
}

func TestUpdateDefaultsHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/defaults", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp model.ApiError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrValidationError, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestUpdateDefaultsHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/defaults", "", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateDefaultsHandler_DoesNotAffectPeriods(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	// The handler calls UpsertDefaults, NOT CreatePeriod or UpdatePeriod.
	repo.On("UpsertDefaults", mock.Anything, mock.AnythingOfType("*model.DefaultSettings")).
		Return(&model.DefaultSettings{
			UserID:            "user-123",
			BudgetAmount:      500000,
			EssentialsPercent: 60,
			DesiresPercent:    20,
			SavingsPercent:    20,
			Currency:          "USD",
		}, nil)

	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/defaults", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 60,
		"desiresPercent":    20,
		"savingsPercent":    20,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify no period-related methods were called
	repo.AssertNotCalled(t, "CreatePeriod", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "GetCurrentPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
