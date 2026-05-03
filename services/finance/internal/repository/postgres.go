package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// PostgresFinanceRepository implements FinanceRepository using sqlc-generated queries.
type PostgresFinanceRepository struct {
	queries *db.Queries
}

// NewPostgresFinanceRepository creates a new PostgresFinanceRepository.
func NewPostgresFinanceRepository(queries *db.Queries) *PostgresFinanceRepository {
	return &PostgresFinanceRepository{queries: queries}
}

func (r *PostgresFinanceRepository) UpsertDefaults(ctx context.Context, settings *model.DefaultSettings) (*model.DefaultSettings, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(settings.UserID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.UpsertDefaults(ctx, db.UpsertDefaultsParams{
		UserID:            uid,
		BudgetAmount:      settings.BudgetAmount,
		EssentialsPercent: settings.EssentialsPercent,
		DesiresPercent:    settings.DesiresPercent,
		SavingsPercent:    settings.SavingsPercent,
		Currency:          settings.Currency,
	})
	if err != nil {
		return nil, err
	}
	return dbDefaultsToModel(row), nil
}

func (r *PostgresFinanceRepository) GetDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.GetDefaults(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbDefaultsToModel(row), nil
}

func (r *PostgresFinanceRepository) CreateTag(ctx context.Context, userID, name string, isDefault bool) (*model.Tag, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.CreateTag(ctx, db.CreateTagParams{
		UserID:    uid,
		Name:      name,
		IsDefault: isDefault,
	})
	if err != nil {
		return nil, err
	}
	return dbTagToModel(row), nil
}

func (r *PostgresFinanceRepository) GetTag(ctx context.Context, tagID, userID string) (*model.Tag, error) {
	tid := pgtype.UUID{}
	if err := tid.Scan(tagID); err != nil {
		return nil, fmt.Errorf("parsing tag ID: %w", err)
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.GetTagByID(ctx, db.GetTagByIDParams{ID: tid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbTagToModel(row), nil
}

func (r *PostgresFinanceRepository) ListTags(ctx context.Context, userID string) ([]*model.Tag, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	rows, err := r.queries.ListTags(ctx, uid)
	if err != nil {
		return nil, err
	}

	tags := make([]*model.Tag, len(rows))
	for i, row := range rows {
		tags[i] = dbTagToModel(row)
	}
	return tags, nil
}

func (r *PostgresFinanceRepository) UpdateTag(ctx context.Context, tagID, userID, name string) (*model.Tag, error) {
	tid := pgtype.UUID{}
	if err := tid.Scan(tagID); err != nil {
		return nil, fmt.Errorf("parsing tag ID: %w", err)
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.UpdateTag(ctx, db.UpdateTagParams{Name: name, ID: tid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbTagToModel(row), nil
}

func (r *PostgresFinanceRepository) DeleteTag(ctx context.Context, tagID, userID string) error {
	tid := pgtype.UUID{}
	if err := tid.Scan(tagID); err != nil {
		return fmt.Errorf("parsing tag ID: %w", err)
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return fmt.Errorf("parsing user ID: %w", err)
	}
	return r.queries.DeleteTag(ctx, db.DeleteTagParams{ID: tid, UserID: uid})
}

func (r *PostgresFinanceRepository) CountUserTags(ctx context.Context, userID string) (int64, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return 0, fmt.Errorf("parsing user ID: %w", err)
	}

	return r.queries.CountUserTags(ctx, uid)
}

func (r *PostgresFinanceRepository) CountTagInProRata(ctx context.Context, tagID, userID string) (int64, error) {
	tid := pgtype.UUID{}
	if err := tid.Scan(tagID); err != nil {
		return 0, fmt.Errorf("parsing tag ID: %w", err)
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return 0, fmt.Errorf("parsing user ID: %w", err)
	}
	return r.queries.CountTagInProRata(ctx, db.CountTagInProRataParams{TagID: tid, UserID: uid})
}

func (r *PostgresFinanceRepository) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.GetCurrentPeriod(ctx, db.GetCurrentPeriodParams{
		UserID: uid,
		Year:   year,
		Month:  month,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) CreatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(period.UserID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	row, err := r.queries.CreatePeriod(ctx, db.CreatePeriodParams{
		UserID:            uid,
		Year:              period.Year,
		Month:             period.Month,
		BudgetAmount:      period.BudgetAmount,
		EssentialsPercent: period.EssentialsPercent,
		DesiresPercent:    period.DesiresPercent,
		SavingsPercent:    period.SavingsPercent,
	})
	if err != nil {
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	rows, err := r.queries.ListPeriods(ctx, uid)
	if err != nil {
		return nil, err
	}

	periods := make([]*model.BudgetPeriod, len(rows))
	for i, row := range rows {
		periods[i] = dbPeriodToModel(row)
	}
	return periods, nil
}

func dbDefaultsToModel(d db.FinanceDefaultSetting) *model.DefaultSettings {
	return &model.DefaultSettings{
		UserID:            formatUUID(d.UserID.Bytes),
		BudgetAmount:      d.BudgetAmount,
		EssentialsPercent: d.EssentialsPercent,
		DesiresPercent:    d.DesiresPercent,
		SavingsPercent:    d.SavingsPercent,
		Currency:          d.Currency,
		CreatedAt:         d.CreatedAt.Time,
		UpdatedAt:         d.UpdatedAt.Time,
	}
}

func dbTagToModel(t db.FinanceTag) *model.Tag {
	return &model.Tag{
		ID:        formatUUID(t.ID.Bytes),
		UserID:    formatUUID(t.UserID.Bytes),
		Name:      t.Name,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt.Time,
		UpdatedAt: t.UpdatedAt.Time,
	}
}

func dbPeriodToModel(p db.FinanceBudgetPeriod) *model.BudgetPeriod {
	return &model.BudgetPeriod{
		ID:                formatUUID(p.ID.Bytes),
		UserID:            formatUUID(p.UserID.Bytes),
		Year:              p.Year,
		Month:             p.Month,
		BudgetAmount:      p.BudgetAmount,
		EssentialsPercent: p.EssentialsPercent,
		DesiresPercent:    p.DesiresPercent,
		SavingsPercent:    p.SavingsPercent,
		CreatedAt:         p.CreatedAt.Time,
		UpdatedAt:         p.UpdatedAt.Time,
	}
}

func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// PostgresTxBeginner implements TxBeginner using pgxpool.
type PostgresTxBeginner struct {
	pool *pgxpool.Pool
}

func NewPostgresTxBeginner(pool *pgxpool.Pool) *PostgresTxBeginner {
	return &PostgresTxBeginner{pool: pool}
}

func (b *PostgresTxBeginner) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	queries := db.New(tx)
	repo := NewPostgresFinanceRepository(queries)
	return &postgresTx{tx: tx, repo: repo}, nil
}

type postgresTx struct {
	tx   pgx.Tx
	repo FinanceRepository
}

func (t *postgresTx) Commit(ctx context.Context) error  { return t.tx.Commit(ctx) }
func (t *postgresTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
func (t *postgresTx) Repo() FinanceRepository            { return t.repo }
