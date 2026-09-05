package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/service"
)

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
	assert.Contains(t, errResp.Fields["essentialsPercent"], "must sum to 100")
}

func TestCompleteOnboardingHandler_MultiFieldValidationEmitsFields(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	// 50/50/50 sums to 150: a multi-field validation failure. The wire
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
	assert.Contains(t, errResp.Fields["essentialsPercent"], "must sum to 100")
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
