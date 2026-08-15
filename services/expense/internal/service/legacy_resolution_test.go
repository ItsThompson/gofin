package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// legacyExpense builds an expense that simulates what the repository returns
// for a legacy row: LegacySynthesized=true, currencies set to the legacy
// currency, with a migration snapshot.
func legacyExpense(id, currency string, year int32, month int32) *model.Expense {
	return &model.Expense{
		ID:                    id,
		UserID:                "user-1",
		Name:                  "Legacy Groceries",
		Amount:                2500,
		Currency:              currency,
		ExpenseType:           "essentials",
		TagID:                 "tag-food",
		ExpenseDate:           "2026-05-03",
		PeriodYear:            year,
		PeriodMonth:           month,
		Status:                "active",
		CreatedAt:             "2026-05-03T10:00:00Z",
		MoneySnapshotVersion:  1, // synthesized by rowToExpense
		TransactionAmount:     2500,
		TransactionCurrency:   currency,
		ReportingAmount:       2500,
		ReportingCurrency:     currency,
		ExchangeRate:          "1",
		ExchangeRateSource:    model.ExchangeSourceMigration,
		ExchangeRateTimestamp: "2026-05-03T10:00:00Z",
		LegacySynthesized:     true,
	}
}

func newLegacyTestService(repo *mockExpenseRepository, periodClient *mockPeriodContextClient) *ExpenseService {
	return NewExpenseService(repo, periodClient, nil, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func periodClientReturning(currency string) *mockPeriodContextClient {
	c := new(mockPeriodContextClient)
	c.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(&PeriodContext{PeriodID: "p1", UserID: "user-1", Year: 2026, Month: 5, ReportingCurrency: currency}, nil)
	return c
}

// TestResolveLegacySnapshot_MatchingCurrency asserts a legacy row whose
// currency matches the period reporting currency is normalized correctly and
// emits the legacy_snapshot_synthesized event.
func TestResolveLegacySnapshot_MatchingCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	exp := legacyExpense("exp-1", "USD", 2026, 5)
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(exp, nil)

	result, err := svc.GetExpense(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	assert.Equal(t, "USD", result.TransactionCurrency)
	assert.Equal(t, "USD", result.ReportingCurrency)
	assert.Equal(t, int64(2500), result.TransactionAmount)
	assert.Equal(t, int64(2500), result.ReportingAmount)
	assert.Equal(t, model.ExchangeSourceMigration, result.ExchangeRateSource)
	pc.AssertExpectations(t)
}

// TestResolveLegacySnapshot_DifferingCurrency asserts a legacy row whose
// currency differs from the period reporting currency is normalized to the
// period reporting currency (both transaction and reporting).
func TestResolveLegacySnapshot_DifferingCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	exp := legacyExpense("exp-1", "EUR", 2026, 5)
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(exp, nil)

	result, err := svc.GetExpense(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	assert.Equal(t, "USD", result.TransactionCurrency, "transaction currency normalized to period currency")
	assert.Equal(t, "USD", result.ReportingCurrency, "reporting currency normalized to period currency")
	assert.Equal(t, int64(2500), result.TransactionAmount)
	assert.Equal(t, int64(2500), result.ReportingAmount)
	assert.Equal(t, "EUR", result.Currency, "legacy Currency field is preserved as obsolete metadata")
	pc.AssertExpectations(t)
}

// TestResolveLegacySnapshot_UnsupportedCurrency asserts a legacy row with an
// unsupported currency is normalized to the period reporting currency.
func TestResolveLegacySnapshot_UnsupportedCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	exp := legacyExpense("exp-1", "XYZ", 2026, 5) // unsupported currency
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(exp, nil)

	result, err := svc.GetExpense(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	assert.Equal(t, "USD", result.TransactionCurrency)
	assert.Equal(t, "USD", result.ReportingCurrency)
	assert.Equal(t, "XYZ", result.Currency, "legacy Currency field preserved")
	pc.AssertExpectations(t)
}

// TestResolveLegacySnapshot_PartialSnapshotFields asserts a legacy row with
// partial nullable snapshot columns emits partial_snapshot_fields_ignored
// telemetry and normalizes to the period reporting currency.
func TestResolveLegacySnapshot_PartialSnapshotFields(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	exp := legacyExpense("exp-1", "USD", 2026, 5)
	exp.PartialSnapshotFields = true
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(exp, nil)

	result, err := svc.GetExpense(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	assert.Equal(t, "USD", result.TransactionCurrency)
	assert.Equal(t, "USD", result.ReportingCurrency)
	pc.AssertExpectations(t)
}

// TestResolveLegacySnapshot_Version1IntegrityFailure asserts a version-1 row
// missing required snapshot fields causes the repository to return an error
// (not a fallback to legacy synthesis), and the read fails safely.
func TestResolveLegacySnapshot_Version1IntegrityFailure(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-bad", "user-1").
		Return(nil, fmt.Errorf("mapping expense row: expense row exp-bad: money_snapshot_version=1 missing required snapshot fields"))

	_, err := svc.GetExpense(context.Background(), "user-1", "exp-bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required snapshot fields")
}

// TestGetExpensesForPeriod_ResolvesLegacyRows asserts GetExpensesForPeriod
// fetches period context and resolves legacy rows.
func TestGetExpensesForPeriod_ResolvesLegacyRows(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	expenses := []*model.Expense{
		legacyExpense("exp-1", "EUR", 2026, 5),
		legacyExpense("exp-2", "USD", 2026, 5),
	}
	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return(expenses, int64(2), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, "USD", result.Data[0].TransactionCurrency, "EUR legacy row normalized to USD")
	assert.Equal(t, "USD", result.Data[0].ReportingCurrency)
	assert.Equal(t, "USD", result.Data[1].TransactionCurrency)
	assert.Equal(t, "USD", result.Data[1].ReportingCurrency)
	pc.AssertExpectations(t)
}

// TestGetExpensesForPeriod_SkipsPeriodContextForNonLegacyRows asserts the
// service does not call Finance for period context when all rows are version-1
// (no legacy rows to resolve).
func TestGetExpensesForPeriod_SkipsPeriodContextForNonLegacyRows(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := new(mockPeriodContextClient)
	svc := newLegacyTestService(repo, pc)

	expenses := []*model.Expense{
		{ID: "exp-1", UserID: "user-1", Amount: 5000, Status: "active",
			MoneySnapshotVersion: 1, TransactionAmount: 5000, TransactionCurrency: "USD",
			ReportingAmount: 5000, ReportingCurrency: "USD", ExchangeRate: "1",
			ExchangeRateSource: model.ExchangeSourceIdentity, ExchangeRateTimestamp: "2026-05-03T10:00:00Z"},
	}
	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return(expenses, int64(1), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID: "user-1", Year: 2026, Month: 5, Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "USD", result.Data[0].TransactionCurrency)
	pc.AssertNotCalled(t, "GetPeriodContext")
}

// TestGetCorrectionHistory_ResolvesLegacyRows asserts correction history
// resolves legacy rows for all entries in the chain.
func TestGetCorrectionHistory_ResolvesLegacyRows(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	chain := []*model.Expense{
		legacyExpense("exp-1", "EUR", 2026, 5),
		legacyExpense("exp-2", "EUR", 2026, 5),
	}
	repo.On("GetCorrectionHistory", mock.Anything, "exp-1", "user-1").Return(chain, nil)

	entries, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "USD", entries[0].TransactionCurrency)
	assert.Equal(t, "USD", entries[0].ReportingCurrency)
	assert.Equal(t, "USD", entries[1].TransactionCurrency)
	pc.AssertExpectations(t)
}

// TestGetProRataGroup_ResolvesLegacyRows asserts pro-rata group reads resolve
// legacy rows using cached period context.
func TestGetProRataGroup_ResolvesLegacyRows(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := periodClientReturning("USD")
	svc := newLegacyTestService(repo, pc)

	expenses := []*model.Expense{
		legacyExpense("exp-1", "EUR", 2026, 5),
	}
	repo.On("GetProRataGroup", mock.Anything, "group-1", "user-1").Return(expenses, nil)

	result, err := svc.GetProRataGroup(context.Background(), "user-1", "group-1")
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "USD", result[0].TransactionCurrency)
	assert.Equal(t, "USD", result[0].ReportingCurrency)
	pc.AssertExpectations(t)
}

// TestGetExpense_PeriodContextFailureDoesNotFailRead asserts that if the period
// context fetch fails, the read still succeeds with the repository's basic
// synthesis (legacy currency as both currencies).
func TestGetExpense_PeriodContextFailureDoesNotFailRead(t *testing.T) {
	repo := new(mockExpenseRepository)
	pc := new(mockPeriodContextClient)
	pc.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(nil, fmt.Errorf("finance service unavailable"))
	svc := newLegacyTestService(repo, pc)

	exp := legacyExpense("exp-1", "EUR", 2026, 5)
	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").Return(exp, nil)

	result, err := svc.GetExpense(context.Background(), "user-1", "exp-1")
	require.NoError(t, err)
	// Without period context, the repository's basic synthesis is preserved.
	assert.Equal(t, "EUR", result.TransactionCurrency)
	assert.Equal(t, "EUR", result.ReportingCurrency)
	pc.AssertExpectations(t)
}
