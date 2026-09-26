package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	financeconfig "github.com/ItsThompson/gofin/services/finance/config"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/metrics"
)

type signallingFinanceContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *signallingFinanceContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

type resultCacheRepo struct {
	repository.FinanceRepository
	period            *model.BudgetPeriod
	periodByID        *model.BudgetPeriod
	periods           []*model.BudgetPeriod
	schedules         []*model.ProRataSchedule
	updatePeriodErr   error
	updatePeriodCalls int
	upcomingCalls     int
	calls             int
}

func (r *resultCacheRepo) GetCurrentPeriod(context.Context, string, int32, int32) (*model.BudgetPeriod, error) {
	r.calls++
	return r.period, nil
}

func (r *resultCacheRepo) GetPeriodByID(context.Context, string, string) (*model.BudgetPeriod, error) {
	return r.periodByID, nil
}

func (r *resultCacheRepo) UpdatePeriod(context.Context, *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	r.updatePeriodCalls++
	if r.updatePeriodErr != nil {
		return nil, r.updatePeriodErr
	}
	return r.period, nil
}

func (r *resultCacheRepo) ListTags(context.Context, string) ([]*model.Tag, error) {
	return nil, nil
}

func (r *resultCacheRepo) ListPeriods(context.Context, string) ([]*model.BudgetPeriod, error) {
	return r.periods, nil
}

func (r *resultCacheRepo) ListHealthScoreScalars(context.Context, string) ([]*model.HealthScoreTrendPoint, error) {
	return nil, nil
}

func (r *resultCacheRepo) GetUpcomingProRata(context.Context, string) ([]*model.ProRataSchedule, error) {
	r.upcomingCalls++
	return r.schedules, nil
}

type resultCacheExpenseClient struct {
	expenses        []ExpenseData
	revision        ExpenseRevision
	revisionErr     error
	revisionStarted chan struct{}
	revisionRelease chan struct{}
	expenseErr      error
	expenseStarted  chan struct{}
	expenseRelease  chan struct{}
	expenseCalls    int
	revisionCalls   int
}

func (c *resultCacheExpenseClient) GetActiveExpensesForPeriod(context.Context, string, int32, int32) ([]ExpenseData, error) {
	c.expenseCalls++
	if c.expenseStarted != nil {
		close(c.expenseStarted)
		<-c.expenseRelease
	}
	if c.expenseErr != nil {
		return nil, c.expenseErr
	}
	return c.expenses, nil
}

func (c *resultCacheExpenseClient) GetExpenseRevision(ctx context.Context, _ string) (ExpenseRevision, error) {
	c.revisionCalls++
	if c.revisionStarted != nil {
		close(c.revisionStarted)
		select {
		case <-c.revisionRelease:
		case <-ctx.Done():
			return ExpenseRevision{}, ctx.Err()
		}
	}
	return c.revision, c.revisionErr
}

func (c *resultCacheExpenseClient) CountExpensesByTag(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (c *resultCacheExpenseClient) CreateExpense(context.Context, CreateExpenseInput) (*CreatedExpenseData, error) {
	return nil, nil
}

func (c *resultCacheExpenseClient) CreateProRataInstallment(context.Context, CreateProRataInstallmentInput) (*CreatedExpenseData, error) {
	return nil, nil
}

func newResultCacheTestService(now *time.Time, repo *resultCacheRepo, expense *resultCacheExpenseClient) *FinanceService {
	config := DefaultResultCacheConfig()
	config.MaxEntries = 32
	config.MaxBytes = 1024 * 1024
	config.MaxEntryBytes = 1024 * 1024
	config.MaxAge = 48 * time.Hour
	config.ValidationLease = 2 * time.Minute
	return NewFinanceServiceWithCache(repo, nil, expense, func() time.Time { return *now }, slog.New(slog.NewJSONHandler(io.Discard, nil)), config)
}

func TestFinanceResultCache_RevisionValidationTimeout(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{revisionStarted: make(chan struct{}), revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	config := DefaultResultCacheConfig()
	config.ValidationTimeout = 10 * time.Millisecond
	svc := NewFinanceServiceWithCache(repo, nil, expense, func() time.Time { return now }, slog.New(slog.NewJSONHandler(io.Discard, nil)), config)

	startedAt := time.Now()
	_, err := svc.GetPeriodSummary(context.Background(), "user-1", 2026, 1)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(startedAt), time.Second)
}

func TestFinanceResultCache_RevisionValidationHonorsCallerCancellation(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{revisionStarted: make(chan struct{}), revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.GetPeriodSummary(ctx, "user-1", 2026, 1)
		errCh <- err
	}()
	<-expense.revisionStarted
	cancel()

	err := <-errCh
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFinanceResultCachesUseAggregateBudget(t *testing.T) {
	totalBytes := int64(512 * 1024 * 1024)
	config := DefaultResultCacheConfig()
	config.MaxEntries = 256
	config.MaxBytes = totalBytes
	caches := newFinanceResultCaches(config, time.Now)

	assert.LessOrEqual(t, int64(financeResultCacheCount)*caches.maxBytesPerCache, financeconfig.MaxFinanceResultCacheBytes)
	assert.LessOrEqual(t, int64(financeResultCacheCount*caches.maxEntriesPerCache), int64(config.MaxEntries))
	assert.LessOrEqual(t, caches.maxEntryBytesPerCache, caches.maxBytesPerCache)
}

func TestFinanceResultCacheKeysIncludeOperationAndCanonicalTrendWindow(t *testing.T) {
	require.NotEqual(t, trendResultKey(operationTrends, "user-1", 2026, 1, 6), trendResultKey(operationTrends, "user-1", 2026, 1, 12))
	require.Equal(t, int32(6), normalizeTrendMonths(0))
	require.Equal(t, int32(6), normalizeTrendMonths(-4))
	require.Equal(t, int32(12), normalizeTrendMonths(99))
	require.Contains(t, dashboardResultKey(operationSummary, "user-1", 2026, 1), "summary|user-1|2026|1")
}

func TestEvictUserCacheLeavesUnrelatedUserWarm(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", Year: 2026, Month: 1}}
	expense := &resultCacheExpenseClient{revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	_, err := svc.GetCurrentPeriod(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetCurrentPeriod(t.Context(), "user-2", 2026, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, repo.calls)

	require.NoError(t, svc.EvictUserCache(t.Context(), "user-1"))
	_, err = svc.GetCurrentPeriod(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetCurrentPeriod(t.Context(), "user-2", 2026, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, repo.calls)
}

func TestFinanceResultCache_IsolatesUsersAndRenewsLease(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100, ExpenseType: "essentials"}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	first, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	second, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, expense.expenseCalls)
	require.Equal(t, 2, expense.revisionCalls)

	_, err = svc.GetPeriodSummary(t.Context(), "user-2", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)

	now = now.Add(3 * time.Minute)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)
	require.Equal(t, 5, expense.revisionCalls)

	now = now.Add(1 * time.Minute)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 5, expense.revisionCalls)

	expense.revision.Revision = 2
	now = now.Add(3 * time.Minute)
	fresh, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(100), fresh.TotalSpent)
	require.Equal(t, 3, expense.expenseCalls)
}

func TestFinanceResultCache_RevisionChangePurgesEveryDependentOperation(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100, TagID: "tag-1", ExpenseType: "essentials"}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)

	expense.expenses[0].ReportingAmount = 200
	expense.revision.Revision = 2
	now = now.Add(3 * time.Minute)
	freshSummary, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(200), freshSummary.TotalSpent)

	freshByTag, err := svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Len(t, freshByTag, 1)
	require.Equal(t, int64(200), freshByTag[0].Amount)
	require.Equal(t, 4, expense.expenseCalls)
}

func TestFinanceResultCache_RevisionChangePurgesAllDependentOperations(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	period := &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, ReportingCurrencyCode: "USD", EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}
	repo := &resultCacheRepo{period: period, periods: []*model.BudgetPeriod{period}, schedules: []*model.ProRataSchedule{{ID: "schedule-1"}}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100, ExpenseType: "essentials", TagID: "tag-1", ExpenseDate: "2026-01-10"}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	byTag, err := svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetCumulativeSpend(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	comparison, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	trends, err := svc.GetSpendingTrends(t.Context(), "user-1", 2026, 1, 6)
	require.NoError(t, err)
	health, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	healthTrend, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 1, 6)
	require.NoError(t, err)
	upcoming, err := svc.GetUpcomingProRata(t.Context(), "user-1")
	require.NoError(t, err)
	require.Equal(t, int64(100), comparison.CurrentSpent)
	require.Equal(t, int64(100), byTag[0].Amount)
	require.Len(t, trends, 6)
	require.NotEmpty(t, health)
	require.Len(t, healthTrend, 1)
	require.Equal(t, 7, expense.expenseCalls)
	require.Equal(t, 1, repo.upcomingCalls)

	expense.expenses[0].ReportingAmount = 200
	expense.revision.Revision = 2
	now = now.Add(3 * time.Minute)
	freshSummary, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	freshByTag, err := svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	freshCumulative, err := svc.GetCumulativeSpend(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	freshComparison, err := svc.GetHistoricalComparison(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	freshTrends, err := svc.GetSpendingTrends(t.Context(), "user-1", 2026, 1, 6)
	require.NoError(t, err)
	freshHealth, err := svc.GetHealthScore(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	freshHealthTrend, err := svc.GetHealthScoreTrend(t.Context(), "user-1", 2026, 1, 6)
	require.NoError(t, err)
	freshUpcoming, err := svc.GetUpcomingProRata(t.Context(), "user-1")
	require.NoError(t, err)

	require.Equal(t, int64(200), freshSummary.TotalSpent)
	require.Equal(t, int64(200), freshByTag[0].Amount)
	require.Equal(t, int64(200), freshCumulative[len(freshCumulative)-1].Actual)
	require.Equal(t, int64(200), freshComparison.CurrentSpent)
	require.Equal(t, int64(200), freshTrends[len(freshTrends)-1].TotalSpent)
	require.NotNil(t, freshHealth)
	require.Len(t, freshHealthTrend, 1)
	require.Equal(t, 14, expense.expenseCalls)
	require.Equal(t, upcoming, freshUpcoming)
	require.Equal(t, 1, repo.upcomingCalls)
}

func TestFinanceResultCache_ForcedReadBypassesCachedResultAndDoesNotFallback(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100, ExpenseType: "essentials"}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	cached, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(100), cached.TotalSpent)

	expense.expenses[0].ReportingAmount = 200
	fresh, err := svc.GetPeriodSummary(WithCacheBypass(t.Context()), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(200), fresh.TotalSpent)
	require.Equal(t, 2, expense.expenseCalls)

	ordinary, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(200), ordinary.TotalSpent)
	require.Equal(t, 2, expense.expenseCalls)

	expense.expenseErr = errors.New("forced source unavailable")
	_, err = svc.GetPeriodSummary(WithCacheBypass(t.Context()), "user-1", 2026, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forced source unavailable")
	require.Equal(t, 3, expense.expenseCalls)

	ordinary, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(200), ordinary.TotalSpent)
	require.Equal(t, 3, expense.expenseCalls)
}

func TestFinanceResultCache_MutationErrorPurgesCachedResults(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	period := &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}
	repo := &resultCacheRepo{period: period, periodByID: period, updatePeriodErr: errors.New("commit status unknown")}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	first, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(100), first.TotalSpent)

	_, err = svc.UpdatePeriod(t.Context(), "user-1", "period-1", &model.UpdatePeriodRequest{
		BudgetAmount:      1000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
	})
	require.Error(t, err)
	require.Equal(t, 1, repo.updatePeriodCalls)

	expense.expenses[0].ReportingAmount = 200
	expense.revision.Revision = 2
	fresh, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, int64(200), fresh.TotalSpent)
	require.Equal(t, 2, expense.expenseCalls)
}

func financeHistogramSampleCount(t *testing.T, metric *prometheus.HistogramVec, operation string) uint64 {
	t.Helper()
	observed, err := metric.GetMetricWithLabelValues(operation)
	require.NoError(t, err)
	written := &dto.Metric{}
	require.NoError(t, observed.(prometheus.Metric).Write(written))
	return written.GetHistogram().GetSampleCount()
}

func TestFinanceResultCacheMetricsUseFiniteOperationNames(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)

	beforeMiss := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "miss"))
	beforeHit := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "hit"))
	beforeFreshnessChecks := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "freshness_check"))
	beforeFreshnessFailures := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "freshness_failure"))
	beforeEvictions := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "eviction"))
	beforeJoin := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "single_flight_join"))
	beforeCapacityBypass := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "capacity_bypass"))
	beforeSourceCount := financeHistogramSampleCount(t, metrics.FinanceReadSourceDuration, operationSummary)

	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	now = now.Add(3 * time.Minute)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	now = now.Add(3 * time.Minute)
	expense.revisionErr = errors.New("expense unavailable")
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.Error(t, err)

	afterMiss := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "miss"))
	afterHit := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "hit"))
	afterFreshnessChecks := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "freshness_check"))
	afterFreshnessFailures := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "freshness_failure"))
	afterEvictions := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "eviction"))

	joinRepo := &resultCacheRepo{period: repo.period}
	joinExpense := &resultCacheExpenseClient{
		expenses:       []ExpenseData{{ReportingAmount: 100}},
		revision:       ExpenseRevision{Epoch: "epoch-1", Revision: 1},
		expenseStarted: make(chan struct{}),
		expenseRelease: make(chan struct{}),
	}
	joinSvc := newResultCacheTestService(&now, joinRepo, joinExpense)
	firstDone := make(chan error, 1)
	go func() {
		_, err := joinSvc.GetPeriodSummary(context.Background(), "user-1", 2026, 1)
		firstDone <- err
	}()
	<-joinExpense.expenseStarted
	secondContext := &signallingFinanceContext{Context: context.Background(), entered: make(chan struct{})}
	secondDone := make(chan error, 1)
	go func() {
		_, err := joinSvc.GetPeriodSummary(secondContext, "user-1", 2026, 1)
		secondDone <- err
	}()
	<-secondContext.entered
	close(joinExpense.expenseRelease)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)

	oversizeRepo := &resultCacheRepo{period: repo.period}
	oversizeExpense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	oversizeConfig := DefaultResultCacheConfig()
	oversizeConfig.MaxEntryBytes = 1
	oversizeSvc := NewFinanceServiceWithCache(oversizeRepo, nil, oversizeExpense, func() time.Time { return now }, slog.New(slog.NewJSONHandler(io.Discard, nil)), oversizeConfig)
	_, err = oversizeSvc.GetPeriodSummary(context.Background(), "user-1", 2026, 1)
	require.NoError(t, err)

	afterJoin := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "single_flight_join"))
	afterCapacityBypass := testutil.ToFloat64(metrics.FinanceResultCacheEventsTotal.WithLabelValues(operationSummary, "capacity_bypass"))
	afterSourceCount := financeHistogramSampleCount(t, metrics.FinanceReadSourceDuration, operationSummary)
	assert.Equal(t, float64(1), afterMiss-beforeMiss)
	assert.Equal(t, float64(2), afterHit-beforeHit)
	assert.Equal(t, float64(4), afterFreshnessChecks-beforeFreshnessChecks)
	assert.Equal(t, float64(1), afterFreshnessFailures-beforeFreshnessFailures)
	assert.Equal(t, float64(1), afterEvictions-beforeEvictions)
	assert.Equal(t, float64(1), afterJoin-beforeJoin)
	assert.Equal(t, float64(1), afterCapacityBypass-beforeCapacityBypass)
	assert.Equal(t, uint64(3), afterSourceCount-beforeSourceCount)
	assert.Equal(t, 1, joinExpense.expenseCalls)
	assert.Equal(t, 1, oversizeExpense.expenseCalls)
}

func TestFinanceResultCache_FailedRevisionCheckFailsClosed(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenses: []ExpenseData{{ReportingAmount: 100}}, revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)
	require.NoError(t, func() error { _, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1); return err }())

	now = now.Add(3 * time.Minute)
	expense.revisionErr = errors.New("expense unavailable")
	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validating expense revision")
	require.Equal(t, 1, expense.expenseCalls)
}

func TestFinanceResultCache_ExpiresAtCurrentDayBoundary(t *testing.T) {
	now := time.Date(2026, 1, 15, 23, 59, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)
	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	now = time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)
}

func TestFinanceResultCache_DoesNotRetainPeriodNotFound(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{}
	expense := &resultCacheExpenseClient{}
	svc := newResultCacheTestService(&now, repo, expense)
	_, err := svc.GetCurrentPeriod(t.Context(), "user-1", 2026, 1)
	require.Error(t, err)
	repo.period = &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1}
	period, err := svc.GetCurrentPeriod(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, "period-1", period.ID)
	require.Equal(t, 2, repo.calls)
}

func TestFinanceResultCache_DisabledReadsSourceEveryTime(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	config := DefaultResultCacheConfig()
	config.Enabled = false
	svc := NewFinanceServiceWithCache(repo, nil, expense, func() time.Time { return now }, slog.New(slog.NewJSONHandler(io.Discard, nil)), config)
	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)
}

func TestFinanceResultCache_DisabledReadRejectsRevisionChangeDuringSourceRead(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{
		expenses:       []ExpenseData{{ReportingAmount: 100}},
		revision:       ExpenseRevision{Epoch: "epoch-1", Revision: 1},
		expenseStarted: make(chan struct{}),
		expenseRelease: make(chan struct{}),
	}
	config := DefaultResultCacheConfig()
	config.Enabled = false
	svc := NewFinanceServiceWithCache(repo, nil, expense, func() time.Time { return now }, slog.New(slog.NewJSONHandler(io.Discard, nil)), config)

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
		errCh <- err
	}()
	<-expense.expenseStarted
	expense.revision.Revision = 2
	close(expense.expenseRelease)

	err := <-errCh
	require.Error(t, err)
	require.Contains(t, err.Error(), "expense revision changed during summary load")
	require.Equal(t, 2, expense.revisionCalls)
}

func TestFinanceResultCache_CachesValidEmptyResults(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)
	first, err := svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	second, err := svc.GetSpendingByTag(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Empty(t, first)
	require.Empty(t, second)
	require.NotNil(t, second)
	require.Equal(t, 1, expense.expenseCalls)
}

func TestFinanceResultCache_DoesNotStoreSourceErrors(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &resultCacheRepo{period: &model.BudgetPeriod{ID: "period-1", UserID: "user-1", Year: 2026, Month: 1, BudgetAmount: 1000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20}}
	expense := &resultCacheExpenseClient{expenseErr: errors.New("source unavailable"), revision: ExpenseRevision{Epoch: "epoch-1", Revision: 1}}
	svc := newResultCacheTestService(&now, repo, expense)
	_, err := svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.Error(t, err)

	expense.expenseErr = nil
	_, err = svc.GetPeriodSummary(t.Context(), "user-1", 2026, 1)
	require.NoError(t, err)
	require.Equal(t, 2, expense.expenseCalls)
}
