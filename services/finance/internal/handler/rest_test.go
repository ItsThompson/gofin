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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
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

func (m *mockFinanceRepository) ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.BudgetPeriod), args.Error(1)
}

func (m *mockFinanceRepository) GetPeriodByID(ctx context.Context, periodID, userID string) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, periodID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockFinanceRepository) UpdatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockFinanceRepository) GetTag(ctx context.Context, tagID, userID string) (*model.Tag, error) {
	args := m.Called(ctx, tagID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockFinanceRepository) UpdateTag(ctx context.Context, tagID, userID, name string) (*model.Tag, error) {
	args := m.Called(ctx, tagID, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Tag), args.Error(1)
}

func (m *mockFinanceRepository) DeleteTag(ctx context.Context, tagID, userID string) error {
	args := m.Called(ctx, tagID, userID)
	return args.Error(0)
}

func (m *mockFinanceRepository) CountTagInProRata(ctx context.Context, tagID, userID string) (int64, error) {
	args := m.Called(ctx, tagID, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockFinanceRepository) GetLatestPeriod(ctx context.Context, userID string) (*model.BudgetPeriod, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.BudgetPeriod), args.Error(1)
}

func (m *mockFinanceRepository) CreateProRataSchedule(ctx context.Context, schedule *model.ProRataSchedule) (*model.ProRataSchedule, error) {
	args := m.Called(ctx, schedule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ProRataSchedule), args.Error(1)
}

func (m *mockFinanceRepository) GetPendingProRata(ctx context.Context, userID string, year, month int32) ([]*model.ProRataSchedule, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.ProRataSchedule), args.Error(1)
}

func (m *mockFinanceRepository) MarkProRataApplied(ctx context.Context, scheduleID string) error {
	args := m.Called(ctx, scheduleID)
	return args.Error(0)
}

func (m *mockFinanceRepository) GetUpcomingProRata(ctx context.Context, userID string) ([]*model.ProRataSchedule, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.ProRataSchedule), args.Error(1)
}

func (m *mockFinanceRepository) DeleteAllUserData(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockFinanceRepository) GetHealthScore(ctx context.Context, userID string, year, month int32) (*model.HealthScore, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.HealthScore), args.Error(1)
}

func (m *mockFinanceRepository) UpsertHealthScore(ctx context.Context, userID string, score *model.HealthScore) (*model.HealthScore, error) {
	args := m.Called(ctx, userID, score)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.HealthScore), args.Error(1)
}

func (m *mockFinanceRepository) ListHealthScoreScalars(ctx context.Context, userID string) ([]*model.HealthScoreTrendPoint, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.HealthScoreTrendPoint), args.Error(1)
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
	financeSvc := service.NewFinanceService(repo, txBeginner, nil, time.Now, logger)

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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestCompleteOnboardingHandler_MultiFieldValidationEmitsFields(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	// 50/50/50 sums to 150: a multi-field validation failure (C6). The wire
	// response must carry field-level detail for every offending percentage,
	// end to end through apierr.Respond.
	w := doJSONWithUserID(r, "POST", "/api/finance/onboarding", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    50,
		"savingsPercent":    50,
		"currency":          "USD",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Equal(t, map[string]string{
		"essentialsPercent": "must sum to 100 with desires and savings",
		"desiresPercent":    "must sum to 100 with essentials and savings",
		"savingsPercent":    "must sum to 100 with essentials and desires",
	}, errResp.Fields)
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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeInternal, errResp.Code)

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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeNotFound, errResp.Code)
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

	var errResp apierr.APIError
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

	repo.On("GetLatestPeriod", mock.Anything, "user-123").Return(nil, nil)
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
	repo.On("GetPendingProRata", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

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

	var resp model.CreatePeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-new", resp.Period.ID)
	assert.Equal(t, int32(2026), resp.Period.Year)
	assert.Equal(t, int32(5), resp.Period.Month)
	assert.Equal(t, int64(300000), resp.Period.BudgetAmount)
}

func TestCreatePeriodHandler_ZeroBudget(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("GetLatestPeriod", mock.Anything, "user-123").Return(nil, nil)
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
	repo.On("GetPendingProRata", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

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

	var resp model.CreatePeriodResponse
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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
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

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
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

// --- Dashboard Aggregation Handler Tests ---

// mockExpenseClient implements service.ExpenseClient for tests.
type mockExpenseClient struct {
	mock.Mock
}

func (m *mockExpenseClient) GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]service.ExpenseData, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]service.ExpenseData), args.Error(1)
}

func (m *mockExpenseClient) CountExpensesByTag(ctx context.Context, userID, tagID string) (int64, error) {
	args := m.Called(ctx, userID, tagID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseClient) CreateExpense(ctx context.Context, req service.CreateExpenseInput) (*service.CreatedExpenseData, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.CreatedExpenseData), args.Error(1)
}

func setupTestRouterWithExpenseClient(repo *mockFinanceRepository, txBeginner *mockTxBeginner, expClient *mockExpenseClient) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, expClient, time.Now, logger)

	h := NewRESTHandler(financeSvc, logger)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func TestGetPeriodSummaryHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{ID: "e1", Amount: 50000, ExpenseType: "essentials", TagID: "t1", ExpenseDate: "2025-01-05"},
			{ID: "e2", Amount: 20000, ExpenseType: "desires", TagID: "t2", ExpenseDate: "2025-01-10"},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.SummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "period-abc", resp.Summary.PeriodID)
	assert.Equal(t, int64(300000), resp.Summary.TotalBudget)
	assert.Equal(t, int64(70000), resp.Summary.TotalSpent)
	assert.Equal(t, int64(230000), resp.Summary.Remaining)
	assert.Equal(t, int64(150000), resp.Summary.Essentials.Allocated)
	assert.Equal(t, int64(50000), resp.Summary.Essentials.Spent)
	assert.Equal(t, int64(90000), resp.Summary.Desires.Allocated)
	assert.Equal(t, int64(20000), resp.Summary.Desires.Spent)
}

func TestGetPeriodSummaryHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(6)).
		Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=6", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
}

func TestGetPeriodSummaryHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", "/api/finance/summary"},
		{"missing month", "/api/finance/summary?year=2025"},
		{"missing year", "/api/finance/summary?month=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSONWithUserID(r, "GET", tt.query, "user-123", nil)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetPeriodSummaryHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/summary?year=2025&month=1", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Health Score Handler Tests ---

func TestGetHealthScoreHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	period := &model.BudgetPeriod{
		ID: "period-h", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}
	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).Return(period, nil)
	repo.On("GetDefaults", mock.Anything, "user-123").
		Return(&model.DefaultSettings{UserID: "user-123", Currency: "USD"}, nil)
	// Only the current period exists, so the stability window is empty.
	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{period}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]service.ExpenseData{
			{ID: "e1", Amount: 140000, ExpenseType: "essentials", ExpenseDate: "2026-05-05"},
			{ID: "e2", Amount: 80000, ExpenseType: "desires", ExpenseDate: "2026-05-06"},
			{ID: "e3", Amount: 40000, ExpenseType: "savings", ExpenseDate: "2026-05-07"},
		}, nil)

	// Fixed clock inside May 2026 -> the target month is provisional (computed
	// live, never persisted), so no stored read or upsert is expected.
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.HealthScore)
	assert.Equal(t, int32(2026), resp.HealthScore.Year)
	assert.Equal(t, int32(5), resp.HealthScore.Month)
	assert.Len(t, resp.HealthScore.Components, 3)
	assert.GreaterOrEqual(t, resp.HealthScore.Total, int32(0))
	assert.LessOrEqual(t, resp.HealthScore.Total, int32(100))
	assert.Equal(t, model.FormulaVersion, resp.HealthScore.FormulaVersion)
	assert.False(t, resp.HealthScore.ConfigureBudget)

	// total must equal the sum of components (AC).
	var sum int32
	for _, component := range resp.HealthScore.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, resp.HealthScore.Total)
}

func TestGetHealthScoreHandler_ConfigureBudget(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "period-0", UserID: "user-123", Year: 2026, Month: 5, BudgetAmount: 0}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.HealthScore)
	assert.True(t, resp.HealthScore.ConfigureBudget)
	assert.Empty(t, resp.HealthScore.Components)
}

func TestGetHealthScoreHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(6)).Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026&month=6", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodNotFound, errResp.Code)
}

func TestGetHealthScoreHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score?year=2026", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHealthScoreTrendHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	periodFor := func(month int32) *model.BudgetPeriod {
		return &model.BudgetPeriod{
			ID: "p", UserID: "user-123", Year: 2026, Month: month,
			BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}
	}
	scalar := func(month, total int32, band string) *model.HealthScoreTrendPoint {
		return &model.HealthScoreTrendPoint{
			Year: 2026, Month: month, Total: total, Band: band,
			Provisional: false, FormulaVersion: model.FormulaVersion,
		}
	}

	repo.On("ListPeriods", mock.Anything, "user-123").
		Return([]*model.BudgetPeriod{periodFor(6), periodFor(5)}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-123").
		Return([]*model.HealthScoreTrendPoint{
			scalar(6, 82, model.HealthBandGreen),
			scalar(5, 70, model.HealthBandAmber),
		}, nil)

	// November 2026: both May and June are closed, so both come from the scalar read.
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026&month=6&months=6", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreTrendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Trends, 2)
	assert.Equal(t, int32(5), resp.Trends[0].Month, "points ascending")
	assert.Equal(t, int32(6), resp.Trends[1].Month)
	assert.Equal(t, int32(70), resp.Trends[0].Total)
	assert.Equal(t, int32(82), resp.Trends[1].Total)
	assert.Equal(t, model.FormulaVersion, resp.Trends[1].FormulaVersion)
}

func TestGetHealthScoreTrendHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHealthScoreTrendHandler_ClampsMonths(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	// months=99 is clamped, not rejected (AC4 "cap 12"). With no periods the trend
	// is empty and the request still succeeds.
	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{}, nil)
	repo.On("ListHealthScoreScalars", mock.Anything, "user-123").Return([]*model.HealthScoreTrendPoint{}, nil)
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, func() time.Time {
		return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	})

	w := doJSONWithUserID(r, "GET", "/api/finance/health-score/trend?year=2026&month=6&months=99", "user-123", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HealthScoreTrendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Trends)
}

func TestGetSpendingByTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{TagID: "tag-food", Amount: 30000},
			{TagID: "tag-food", Amount: 20000},
			{TagID: "tag-bills", Amount: 50000},
		}, nil)

	repo.On("ListTags", mock.Anything, "user-123").
		Return([]*model.Tag{
			{ID: "tag-food", Name: "Food"},
			{ID: "tag-bills", Name: "Bills"},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/by-tag?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagSpendingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.TagSpending, 2)
	// Sorted by amount descending: Food (50000), Bills (50000). Bills first alphabetically? No, Bills = 50000, Food = 50000.
	// Actually: food = 30000+20000 = 50000, bills = 50000. Same amount.
	// But sort is by amount desc. They have same amount. Order is map-iteration dependent.
	// Check that both exist and amounts are correct.
	var foodTag, billsTag model.TagSpending
	for _, tag := range resp.TagSpending {
		switch tag.TagName {
		case "Food":
			foodTag = tag
		case "Bills":
			billsTag = tag
		}
	}
	assert.Equal(t, int64(50000), foodTag.Amount)
	assert.Equal(t, int64(50000), billsTag.Amount)
	assert.InDelta(t, 50.0, foodTag.PercentOfTotal, 0.01)
	assert.InDelta(t, 50.0, billsTag.PercentOfTotal, 0.01)
}

func TestGetSpendingByTagHandler_NoExpenses(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:   "period-abc",
			Year: 2025, Month: 1,
			BudgetAmount:      300000,
			EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{}, nil)

	repo.On("ListTags", mock.Anything, "user-123").
		Return([]*model.Tag{}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/by-tag?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagSpendingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.TagSpending)
}

func TestGetCumulativeSpendHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return(&model.BudgetPeriod{
			ID:                "period-abc",
			UserID:            "user-123",
			Year:              2025,
			Month:             1,
			BudgetAmount:      310000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2025), int32(1)).
		Return([]service.ExpenseData{
			{ExpenseDate: "2025-01-01", Amount: 10000},
			{ExpenseDate: "2025-01-03", Amount: 20000},
		}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/cumulative?year=2025&month=1", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.CumulativeSpendResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Points, 31) // January has 31 days
	assert.Equal(t, int64(10000), resp.Points[0].Actual) // day 1
	assert.Equal(t, int64(10000), resp.Points[1].Actual) // day 2 (carry forward)
	assert.Equal(t, int64(30000), resp.Points[2].Actual) // day 3
	assert.Equal(t, int64(10000), resp.Points[0].Ideal)  // 310000/31*1 = 10000
}

func TestGetCumulativeSpendHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/cumulative", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Tag CRUD Handler Tests ---

func TestListTagsHandler_LazySeeds(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	txRepo := new(mockFinanceRepository)
	tx := &mockTx{repo: txRepo}

	// User has 0 tags: triggers lazy seeding
	repo.On("CountUserTags", mock.Anything, "user-1").Return(int64(0), nil)
	txBeginner.On("BeginTx", mock.Anything).Return(tx, nil)
	tx.On("Rollback", mock.Anything).Return(nil)
	tx.On("Commit", mock.Anything).Return(nil)

	for _, tagName := range service.DefaultTags {
		txRepo.On("CreateTag", mock.Anything, "user-1", tagName, true).
			Return(&model.Tag{ID: "t-" + tagName, Name: tagName, IsDefault: true}, nil)
	}

	repo.On("ListTags", mock.Anything, "user-1").
		Return([]*model.Tag{{ID: "t-Bills", Name: "Bills", IsDefault: true}}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "GET", "/api/finance/tags", "user-1", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Tags, 1)
	assert.Equal(t, "Bills", resp.Tags[0].Name)
	tx.AssertCalled(t, "Commit", mock.Anything)
}

func TestListTagsHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/tags", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("CreateTag", mock.Anything, "user-1", "Groceries", false).
		Return(&model.Tag{ID: "tag-new", Name: "Groceries", IsDefault: false}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", map[string]interface{}{
		"name": "Groceries",
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.TagResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Groceries", resp.Tag.Name)
	assert.False(t, resp.Tag.IsDefault)
}

func TestCreateTagHandler_DuplicateName(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	// Simulate PG unique constraint violation
	pgErr := &pgconn.PgError{Code: "23505"}
	repo.On("CreateTag", mock.Anything, "user-1", "Bills", false).
		Return(nil, pgErr)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", map[string]interface{}{
		"name": "Bills",
	})

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDuplicateTag, errResp.Code)
}

func TestCreateTagHandler_MissingBody(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "user-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "POST", "/api/finance/tags", "", map[string]interface{}{"name": "Test"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("UpdateTag", mock.Anything, "tag-1", "user-1", "Renamed").
		Return(&model.Tag{ID: "tag-1", Name: "Renamed", IsDefault: true}, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/tag-1", "user-1", map[string]interface{}{
		"name": "Renamed",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.TagResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Renamed", resp.Tag.Name)
}

func TestUpdateTagHandler_NotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)

	repo.On("UpdateTag", mock.Anything, "nonexistent", "user-1", "Test").
		Return(nil, nil)

	r := setupTestRouter(repo, txBeginner)
	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/nonexistent", "user-1", map[string]interface{}{
		"name": "Test",
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "PUT", "/api/finance/tags/tag-1", "", map[string]interface{}{"name": "X"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteTagHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-c", "user-1").
		Return(&model.Tag{ID: "tag-c", Name: "Custom", IsDefault: false}, nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-c").
		Return(int64(0), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-c", "user-1").
		Return(int64(0), nil)
	repo.On("DeleteTag", mock.Anything, "tag-c", "user-1").
		Return(nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "user-1", nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	repo.AssertCalled(t, "DeleteTag", mock.Anything, "tag-c", "user-1")
}

func TestDeleteTagHandler_DefaultTagForbidden(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-bills", "user-1").
		Return(&model.Tag{ID: "tag-bills", Name: "Bills", IsDefault: true}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-bills", "user-1", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrDefaultTag, errResp.Code)
	assert.Contains(t, errResp.Message, "Default tags cannot be deleted")
}

func TestDeleteTagHandler_TagInUse(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetTag", mock.Anything, "tag-c", "user-1").
		Return(&model.Tag{ID: "tag-c", Name: "Custom", IsDefault: false}, nil)
	expClient.On("CountExpensesByTag", mock.Anything, "user-1", "tag-c").
		Return(int64(5), nil)
	repo.On("CountTagInProRata", mock.Anything, "tag-c", "user-1").
		Return(int64(0), nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)
	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "user-1", nil)

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrTagInUse, errResp.Code)
	assert.Contains(t, errResp.Message, "5 expense(s)")
}

func TestDeleteTagHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "DELETE", "/api/finance/tags/tag-c", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- UpdatePeriod Handler Tests ---

func setupTestRouterWithNowFunc(repo *mockFinanceRepository, txBeginner *mockTxBeginner, expClient *mockExpenseClient, nowFunc func() time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	financeSvc := service.NewFinanceService(repo, txBeginner, expClient, nowFunc, logger)

	h := NewRESTHandler(financeSvc, logger)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func TestUpdatePeriodHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	// Lock to May 2026
	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

	repo.On("GetPeriodByID", mock.Anything, "period-abc", "user-123").Return(&model.BudgetPeriod{
		ID: "period-abc", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}, nil)

	repo.On("UpdatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(&model.BudgetPeriod{
		ID: "period-abc", UserID: "user-123", Year: 2026, Month: 5,
		BudgetAmount: 500000, EssentialsPercent: 60, DesiresPercent: 20, SavingsPercent: 20,
	}, nil)

	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 60,
		"desiresPercent":    20,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.PeriodResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(500000), resp.Period.BudgetAmount)
	assert.Equal(t, int32(60), resp.Period.EssentialsPercent)
}

func TestUpdatePeriodHandler_PastPeriodLocked(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

	// Period is from April (past month)
	repo.On("GetPeriodByID", mock.Anything, "period-old", "user-123").Return(&model.BudgetPeriod{
		ID: "period-old", UserID: "user-123", Year: 2026, Month: 4,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	}, nil)

	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-old", "user-123", map[string]interface{}{
		"budgetAmount":      500000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusForbidden, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, model.ErrPeriodLocked, errResp.Code)
}

func TestUpdatePeriodHandler_InvalidSplit(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "user-123", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    19, // sums to 99
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp apierr.APIError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, apierr.CodeValidation, errResp.Code)
	assert.Contains(t, errResp.Message, "sum to 100%")
}

func TestUpdatePeriodHandler_MissingUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	nowFunc := func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
	r := setupTestRouterWithNowFunc(repo, txBeginner, expClient, nowFunc)

	w := doJSONWithUserID(r, "PUT", "/api/finance/periods/period-abc", "", map[string]interface{}{
		"budgetAmount":      300000,
		"essentialsPercent": 50,
		"desiresPercent":    30,
		"savingsPercent":    20,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GetHistoricalComparison Handler Tests ---

func TestGetHistoricalComparisonHandler_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{
			ID: "p5", UserID: "user-123", Year: 2026, Month: 5,
			BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		}, nil)

	repo.On("ListPeriods", mock.Anything, "user-123").Return([]*model.BudgetPeriod{
		{ID: "p5", UserID: "user-123", Year: 2026, Month: 5, BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
		{ID: "p4", UserID: "user-123", Year: 2026, Month: 4, BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20},
	}, nil)

	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return([]service.ExpenseData{{Amount: 80000}}, nil)
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-123", int32(2026), int32(4)).
		Return([]service.ExpenseData{{Amount: 60000}}, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.HistoricalComparisonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(80000), resp.Comparison.CurrentSpent)
	assert.Equal(t, int64(60000), resp.Comparison.PreviousSpent)
	assert.Nil(t, resp.Comparison.RollingAverage) // only 1 prior period
	assert.InDelta(t, 33.33, resp.Comparison.ChangePercent, 0.01)
}

func TestGetHistoricalComparisonHandler_MissingParams(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)
	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison", "user-123", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistoricalComparisonHandler_PeriodNotFound(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	expClient := new(mockExpenseClient)

	repo.On("GetCurrentPeriod", mock.Anything, "user-123", int32(2026), int32(5)).
		Return(nil, nil)

	r := setupTestRouterWithExpenseClient(repo, txBeginner, expClient)

	w := doJSONWithUserID(r, "GET", "/api/finance/spending/comparison?year=2026&month=5", "user-123", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
