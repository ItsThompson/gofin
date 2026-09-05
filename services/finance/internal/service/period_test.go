package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// fixedNow returns a nowFunc that always returns the given time.
func fixedNow(year int, month time.Month, day int) func() time.Time {
	return func() time.Time {
		return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	}
}

func makePeriod(id string, year int32, month int32) *model.BudgetPeriod {
	return &model.BudgetPeriod{
		ID:                    id,
		UserID:                "user-1",
		Year:                  year,
		Month:                 month,
		BudgetAmount:          300000,
		ReportingCurrencyCode: "USD",
		EssentialsPercent:     50,
		DesiresPercent:        30,
		SavingsPercent:        20,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

// --- Default Settings Tests ---

func TestUpdateDefaults_RejectsUnsupportedCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	result, err := svc.UpdateDefaults(context.Background(), "user-1", &model.UpdateDefaultsRequest{
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "XYZ",
	})

	assert.Nil(t, result)
	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrUnsupportedCurrency, svcErr.Code)
	assert.Equal(t, "unsupported currency", svcErr.Fields["currency"])
	repo.AssertNotCalled(t, "UpsertDefaults", mock.Anything, mock.Anything)
}

func TestUpdateDefaults_DoesNotUpdateExistingPeriods(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	repo.On("UpsertDefaults", mock.Anything, mock.MatchedBy(func(settings *model.DefaultSettings) bool {
		return settings.UserID == "user-1" && settings.Currency == "JPY"
	})).Return(&model.DefaultSettings{
		UserID:            "user-1",
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "JPY",
	}, nil)

	result, err := svc.UpdateDefaults(context.Background(), "user-1", &model.UpdateDefaultsRequest{
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "jpy",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "JPY", result.Currency)
	repo.AssertNotCalled(t, "UpdatePeriod", mock.Anything, mock.Anything)
}

// --- UpdatePeriod Tests ---

func TestUpdatePeriod_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestServiceNow(repo, txBeg, nil, fixedNow(2026, 5, 15))

	existing := makePeriod("period-1", 2026, 5)
	repo.On("GetPeriodByID", mock.Anything, "period-1", "user-1").Return(existing, nil)

	updated := &model.BudgetPeriod{
		ID:                    "period-1",
		UserID:                "user-1",
		Year:                  2026,
		Month:                 5,
		BudgetAmount:          500000,
		ReportingCurrencyCode: "USD",
		EssentialsPercent:     60,
		DesiresPercent:        20,
		SavingsPercent:        20,
		CreatedAt:             existing.CreatedAt,
		UpdatedAt:             time.Now(),
	}
	repo.On("UpdatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(updated, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-1", &model.UpdatePeriodRequest{
		BudgetAmount:      500000,
		EssentialsPercent: 60,
		DesiresPercent:    20,
		SavingsPercent:    20,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(500000), result.BudgetAmount)
	assert.Equal(t, int32(60), result.EssentialsPercent)
}

func TestUpdatePeriod_PastPeriodLocked(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	// Current time is May 2026, trying to update April 2026 period
	svc := newTagTestServiceNow(repo, txBeg, nil, fixedNow(2026, 5, 15))

	existing := makePeriod("period-old", 2026, 4)
	repo.On("GetPeriodByID", mock.Anything, "period-old", "user-1").Return(existing, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-old", &model.UpdatePeriodRequest{
		BudgetAmount:      500000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodLocked, svcErr.Code)
	assert.Equal(t, 403, svcErr.Status)
	assert.Contains(t, svcErr.Message, "read-only")
}

func TestUpdatePeriod_PastYearLocked(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	// Current time is Jan 2026, trying to update Dec 2025 period
	svc := newTagTestServiceNow(repo, txBeg, nil, fixedNow(2026, 1, 10))

	existing := makePeriod("period-old-year", 2025, 12)
	repo.On("GetPeriodByID", mock.Anything, "period-old-year", "user-1").Return(existing, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-old-year", &model.UpdatePeriodRequest{
		BudgetAmount:      500000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodLocked, svcErr.Code)
}

func TestUpdatePeriod_InvalidSplit(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-1", &model.UpdatePeriodRequest{
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    19, // Sums to 99
	})

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Contains(t, svcErr.Fields["essentialsPercent"], "must sum to 100")
}

func TestUpdatePeriod_NegativeBudget(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-1", &model.UpdatePeriodRequest{
		BudgetAmount:      -100,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Contains(t, svcErr.Fields["budgetAmount"], "non-negative")
}

func TestUpdatePeriod_NotFound(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestServiceNow(repo, txBeg, nil, fixedNow(2026, 5, 15))

	repo.On("GetPeriodByID", mock.Anything, "nonexistent", "user-1").Return(nil, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "nonexistent", &model.UpdatePeriodRequest{
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})

	assert.Nil(t, result)
	require.Error(t, err)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestUpdatePeriod_ZeroBudgetAllowed(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestServiceNow(repo, txBeg, nil, fixedNow(2026, 5, 15))

	existing := makePeriod("period-1", 2026, 5)
	repo.On("GetPeriodByID", mock.Anything, "period-1", "user-1").Return(existing, nil)

	updated := &model.BudgetPeriod{
		ID:                    "period-1",
		UserID:                "user-1",
		Year:                  2026,
		Month:                 5,
		BudgetAmount:          0,
		ReportingCurrencyCode: "USD",
		EssentialsPercent:     50,
		DesiresPercent:        30,
		SavingsPercent:        20,
	}
	repo.On("UpdatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(updated, nil)

	result, err := svc.UpdatePeriod(context.Background(), "user-1", "period-1", &model.UpdatePeriodRequest{
		BudgetAmount:      0,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.BudgetAmount)
}
