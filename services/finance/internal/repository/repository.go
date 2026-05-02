package repository

import (
	"context"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// FinanceRepository defines the data access contract for finance operations.
type FinanceRepository interface {
	UpsertDefaults(ctx context.Context, settings *model.DefaultSettings) (*model.DefaultSettings, error)
	GetDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error)
	CreateTag(ctx context.Context, userID, name string, isDefault bool) (*model.Tag, error)
	ListTags(ctx context.Context, userID string) ([]*model.Tag, error)
	CountUserTags(ctx context.Context, userID string) (int64, error)
	GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error)
	CreatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error)
	ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error)
}

// TxBeginner abstracts the ability to begin a transaction for use in service layer.
type TxBeginner interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx represents a transaction with commit/rollback and access to queries.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Repo() FinanceRepository
}
