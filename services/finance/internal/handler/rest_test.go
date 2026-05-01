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
