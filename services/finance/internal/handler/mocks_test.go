package handler

import (
	"context"

	"github.com/stretchr/testify/mock"

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

func (m *mockFinanceRepository) MarkProRataFailed(ctx context.Context, scheduleID string, failureReason string) error {
	args := m.Called(ctx, scheduleID, failureReason)
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

func (m *mockExpenseClient) CreateProRataInstallment(ctx context.Context, req service.CreateProRataInstallmentInput) (*service.CreatedExpenseData, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.CreatedExpenseData), args.Error(1)
}

// mockFxClient implements service.FxClient for handler tests.
type mockFxClient struct {
	mock.Mock
}

func (m *mockFxClient) CaptureRateSnapshot(ctx context.Context, req service.FxCaptureRequest) (*model.CapturedRateSnapshot, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CapturedRateSnapshot), args.Error(1)
}
