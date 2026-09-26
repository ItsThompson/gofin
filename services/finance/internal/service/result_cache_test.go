package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
)

type resultCacheRepo struct {
	repository.FinanceRepository
	period            *model.BudgetPeriod
	periodByID        *model.BudgetPeriod
	updatePeriodErr   error
	updatePeriodCalls int
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

type resultCacheExpenseClient struct {
	expenses       []ExpenseData
	revision       ExpenseRevision
	revisionErr    error
	expenseErr     error
	expenseStarted chan struct{}
	expenseRelease chan struct{}
	expenseCalls   int
	revisionCalls  int
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

func (c *resultCacheExpenseClient) GetExpenseRevision(context.Context, string) (ExpenseRevision, error) {
	c.revisionCalls++
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

func TestFinanceResultCacheKeysIncludeOperationAndCanonicalTrendWindow(t *testing.T) {
	require.NotEqual(t, trendResultKey(operationTrends, "user-1", 2026, 1, 6), trendResultKey(operationTrends, "user-1", 2026, 1, 12))
	require.Equal(t, int32(6), normalizeTrendMonths(0))
	require.Equal(t, int32(6), normalizeTrendMonths(-4))
	require.Equal(t, int32(12), normalizeTrendMonths(99))
	require.Contains(t, dashboardResultKey(operationSummary, "user-1", 2026, 1), "summary|user-1|2026|1")
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
