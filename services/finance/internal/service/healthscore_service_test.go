package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// nowJuly closes the May 2026 target month (May < July), exercising the
// persistence path.
var nowJuly = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

// healthPeriodMonth builds a standard 300000/50/30/20 period for a given month,
// used to populate the ListPeriods window in service tests.
func healthPeriodMonth(year, month int32) *model.BudgetPeriod {
	return &model.BudgetPeriod{
		ID: "p", UserID: "user-1", Year: year, Month: month,
		BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrencyCode: "USD",
	}
}

func TestGetHealthScore_ConfigureBudget(t *testing.T) {
	// budget_amount = 0 -> configure-budget response, no components.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(&model.BudgetPeriod{ID: "p0", UserID: "user-1", Year: 2026, Month: 5, BudgetAmount: 0, ReportingCurrencyCode: "USD"}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.True(t, score.ConfigureBudget)
	assert.Empty(t, score.Components)
	assert.Equal(t, int32(2026), score.Year)
	assert.Equal(t, int32(5), score.Month)
	assert.Equal(t, "USD", score.ReportingCurrencyCode)
	// Must not read defaults, expenses, or storage when there is no budget.
	repo.AssertNotCalled(t, "GetDefaults", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "GetHealthScore", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "GetActiveExpensesForPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScore_Success_UsesPeriodReportingCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	eurPeriod := healthPeriod(300000, 50, 30, 20)
	eurPeriod.ReportingCurrencyCode = "EUR"
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(eurPeriod, nil)
	// Only the current (provisional) period exists, so the stability window is
	// empty (building baseline).
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 130000),
			healthExpense("desires", 90000),
			healthExpense("savings", 30000),
		}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, int32(79), score.Total)
	assert.Equal(t, model.HealthKeySavings, score.Insight.Driver)
	// Currency comes from the period's reporting currency, not default settings.
	assert.Equal(t, "EUR", score.ReportingCurrencyCode)
	assert.Contains(t, score.Insight.Nudge, "€300")
	// Must not read defaults: the period reporting currency is authoritative.
	repo.AssertNotCalled(t, "GetDefaults", mock.Anything, mock.Anything)
	// The provisional current month is never stored.
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScore_EmptyReportingCurrencyFallback(t *testing.T) {
	// A period with no reporting currency (edge case for pre-migration data)
	// falls back to USD.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	period := healthPeriod(300000, 50, 30, 20)
	period.ReportingCurrencyCode = ""
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(period, nil)
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 130000),
			healthExpense("desires", 90000),
			healthExpense("savings", 30000),
		}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Contains(t, score.Insight.Nudge, "$300")
	assert.Equal(t, "USD", score.ReportingCurrencyCode)
}

func TestGetHealthScore_PeriodNotFound(t *testing.T) {
	// no period -> PERIOD_NOT_FOUND (404).
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(6)).Return(nil, nil)

	_, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 6)

	apiErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodNotFound, apiErr.Code)
}

// --- Persistence path ---

func storedScore(version int32) *model.HealthScore {
	return &model.HealthScore{
		Year: 2026, Month: 5, Total: 88, Band: model.HealthBandGreen,
		Provisional: false, FormulaVersion: version,
		Components: []model.HealthComponent{
			{Key: model.HealthKeySavings, Score: 25, Max: 25, Detail: "Saved $600 of $600 target"},
			{Key: model.HealthKeyBudget, Score: 25, Max: 25, Detail: "Spent $2,000 of $2,400 plan"},
			{Key: model.HealthKeyAllocation, Score: 30, Max: 30, Detail: "Balanced across categories"},
			{Key: model.HealthKeyStability, Score: 8, Max: 20, Detail: "Desires spend varied ~60% month to month"},
		},
		Insight: model.HealthInsight{
			Summary: "Spending stability is the softest score this month.",
			Driver:  model.HealthKeyStability,
			Nudge:   "Steadier discretionary spending month to month could lift your score about 12 points.",
		},
	}
}

func TestGetHealthScore_ClosedStoredCurrentVersionReturnsStored(t *testing.T) {
	// A stored, current-version closed month is returned without recompute and
	// without any expense read, insight included.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJuly })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetHealthScore", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(storedScore(model.FormulaVersion), nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, int32(88), score.Total)
	// Insight comes back intact from the stored payload.
	assert.Equal(t, "Spending stability is the softest score this month.", score.Insight.Summary)
	assert.NotEmpty(t, score.Insight.Nudge)
	// Stored hit: no recompute, no expense read, no upsert.
	repo.AssertNotCalled(t, "GetDefaults", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "ListPeriods", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "GetActiveExpensesForPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetHealthScore_ClosedStoredBackfillsReportingCurrency(t *testing.T) {
	// A stored score persisted before reportingCurrency was added gets backfilled
	// from the period's immutable reporting currency.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJuly })

	stored := storedScore(model.FormulaVersion)
	stored.ReportingCurrencyCode = "" // persisted before the field existed
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetHealthScore", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(stored, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, "USD", score.ReportingCurrencyCode, "backfilled from period reporting currency")
}

func TestGetHealthScore_ClosedMissComputesAndUpserts(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJuly })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetHealthScore", mock.Anything, "user-1", int32(2026), int32(5)).Return(nil, nil) // miss
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{healthExpense("desires", 80000)}, nil)
	repo.On("UpsertHealthScore", mock.Anything, "user-1", mock.Anything).Return(nil, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, model.FormulaVersion, score.FormulaVersion)
	assert.False(t, score.Provisional, "a closed month is not provisional")
	assert.Equal(t, "USD", score.ReportingCurrencyCode)
	repo.AssertCalled(t, "UpsertHealthScore", mock.Anything, "user-1", mock.Anything)
}

func TestGetHealthScore_StaleVersionRecomputesAndUpserts(t *testing.T) {
	// A stored stale-version row is recomputed to the current version and upserted.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return nowJuly })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("GetHealthScore", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(storedScore(1), nil) // stale version
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{healthExpense("desires", 80000)}, nil)
	repo.On("UpsertHealthScore", mock.Anything, "user-1", mock.Anything).Return(nil, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, model.FormulaVersion, score.FormulaVersion, "stale row recomputed to current version")
	repo.AssertCalled(t, "UpsertHealthScore", mock.Anything, "user-1", mock.Anything)
}

func TestGetHealthScore_ProvisionalNotStored(t *testing.T) {
	// The provisional current month is computed live and never read from or
	// written to storage.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{healthExpense("desires", 80000)}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.True(t, score.Provisional)
	repo.AssertNotCalled(t, "GetHealthScore", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpsertHealthScore", mock.Anything, mock.Anything, mock.Anything)
}

// --- Stability window feed ---

func TestGetHealthScore_StabilityPresentReadsWindowOncePerMonth(t *testing.T) {
	// With four prior closed months of desires, stability is present. Each
	// windowed period's desires are read exactly once, and the target month once.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil)
	// DESC order: current May (provisional, excluded), then Apr..Jan closed.
	repo.On("ListPeriods", mock.Anything, "user-1").Return([]*model.BudgetPeriod{
		healthPeriodMonth(2026, 5),
		healthPeriodMonth(2026, 4),
		healthPeriodMonth(2026, 3),
		healthPeriodMonth(2026, 2),
		healthPeriodMonth(2026, 1),
	}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 140000),
			healthExpense("desires", 80000),
			healthExpense("savings", 40000),
		}, nil)
	for month := int32(1); month <= 4; month++ {
		expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), month).
			Return([]ExpenseData{healthExpense("desires", 80000)}, nil)
	}

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	require.Len(t, score.Components, 4, "stability is present with >= 3 closed months")
	assert.Equal(t, model.HealthKeyStability, score.Components[3].Key)
	assert.Equal(t, int32(20), score.Components[3].Score, "steady desires earn full stability marks")
	// one read for the target plus one per windowed month (Apr..Jan) = 5.
	expClient.AssertNumberOfCalls(t, "GetActiveExpensesForPeriod", 5)
}

// --- Health-score currency stability ---

func TestGetHealthScore_StableAfterDefaultCurrencyChange(t *testing.T) {
	// A period created with USD reporting currency must keep its USD health-score
	// display even after the user changes their default settings currency to EUR.
	// The score is computed from the period's immutable reporting currency, not
	// from default settings.
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, func() time.Time { return currentMonthNow })

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return(healthPeriod(300000, 50, 30, 20), nil) // reportingCurrency = USD
	repo.On("ListPeriods", mock.Anything, "user-1").
		Return([]*model.BudgetPeriod{healthPeriodMonth(2026, 5)}, nil)
	expClient.On("GetActiveExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]ExpenseData{
			healthExpense("essentials", 130000),
			healthExpense("desires", 90000),
			healthExpense("savings", 30000),
		}, nil)

	score, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, "USD", score.ReportingCurrencyCode)
	assert.Contains(t, score.Insight.Nudge, "$300")
	// Default settings are never read for currency resolution.
	repo.AssertNotCalled(t, "GetDefaults", mock.Anything, mock.Anything)
}
