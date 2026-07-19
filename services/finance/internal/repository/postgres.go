package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/pgutil"
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
	uid, err := pgutil.ParseUUID(settings.UserID)
	if err != nil {
		return nil, err
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
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetDefaults(ctx, uid)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbDefaultsToModel(row), nil
}

func (r *PostgresFinanceRepository) CreateTag(ctx context.Context, userID, name string, isDefault bool) (*model.Tag, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
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
	tid, err := pgutil.ParseUUID(tagID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetTagByID(ctx, db.GetTagByIDParams{ID: tid, UserID: uid})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbTagToModel(row), nil
}

func (r *PostgresFinanceRepository) ListTags(ctx context.Context, userID string) ([]*model.Tag, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
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
	tid, err := pgutil.ParseUUID(tagID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.UpdateTag(ctx, db.UpdateTagParams{Name: name, ID: tid, UserID: uid})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbTagToModel(row), nil
}

func (r *PostgresFinanceRepository) DeleteTag(ctx context.Context, tagID, userID string) error {
	tid, err := pgutil.ParseUUID(tagID)
	if err != nil {
		return err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.DeleteTag(ctx, db.DeleteTagParams{ID: tid, UserID: uid})
}

func (r *PostgresFinanceRepository) CountUserTags(ctx context.Context, userID string) (int64, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return 0, err
	}

	return r.queries.CountUserTags(ctx, uid)
}

func (r *PostgresFinanceRepository) CountTagInProRata(ctx context.Context, tagID, userID string) (int64, error) {
	tid, err := pgutil.ParseUUID(tagID)
	if err != nil {
		return 0, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return 0, err
	}
	return r.queries.CountTagInProRata(ctx, db.CountTagInProRataParams{TagID: tid, UserID: uid})
}

func (r *PostgresFinanceRepository) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetCurrentPeriod(ctx, db.GetCurrentPeriodParams{
		UserID: uid,
		Year:   year,
		Month:  month,
	})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) CreatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	uid, err := pgutil.ParseUUID(period.UserID)
	if err != nil {
		return nil, err
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

func (r *PostgresFinanceRepository) GetPeriodByID(ctx context.Context, periodID, userID string) (*model.BudgetPeriod, error) {
	pid, err := pgutil.ParseUUID(periodID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetPeriodByID(ctx, db.GetPeriodByIDParams{
		ID:     pid,
		UserID: uid,
	})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) UpdatePeriod(ctx context.Context, period *model.BudgetPeriod) (*model.BudgetPeriod, error) {
	pid, err := pgutil.ParseUUID(period.ID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(period.UserID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.UpdatePeriod(ctx, db.UpdatePeriodParams{
		BudgetAmount:      period.BudgetAmount,
		EssentialsPercent: period.EssentialsPercent,
		DesiresPercent:    period.DesiresPercent,
		SavingsPercent:    period.SavingsPercent,
		ID:                pid,
		UserID:            uid,
	})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
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

func (r *PostgresFinanceRepository) GetLatestPeriod(ctx context.Context, userID string) (*model.BudgetPeriod, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetLatestPeriod(ctx, uid)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbPeriodToModel(row), nil
}

func (r *PostgresFinanceRepository) CreateProRataSchedule(ctx context.Context, schedule *model.ProRataSchedule) (*model.ProRataSchedule, error) {
	uid, err := pgutil.ParseUUID(schedule.UserID)
	if err != nil {
		return nil, err
	}
	groupID, err := pgutil.ParseUUID(schedule.ProRataGroup)
	if err != nil {
		return nil, err
	}
	tagID, err := pgutil.ParseUUID(schedule.TagID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateProRataSchedule(ctx, db.CreateProRataScheduleParams{
		UserID:           uid,
		ProRataGroup:     groupID,
		Name:             schedule.Name,
		Amount:           schedule.Amount,
		Currency:         schedule.Currency,
		ExpenseType:      schedule.ExpenseType,
		TagID:            tagID,
		TargetYear:       schedule.TargetYear,
		TargetMonth:      schedule.TargetMonth,
		InstallmentIndex: schedule.InstallmentIndex,
		InstallmentTotal: schedule.InstallmentTotal,
	})
	if err != nil {
		return nil, err
	}
	return dbScheduleToModel(row), nil
}

func (r *PostgresFinanceRepository) GetPendingProRata(ctx context.Context, userID string, year, month int32) ([]*model.ProRataSchedule, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.GetPendingProRata(ctx, db.GetPendingProRataParams{
		UserID:      uid,
		TargetYear:  year,
		TargetMonth: month,
	})
	if err != nil {
		return nil, err
	}

	schedules := make([]*model.ProRataSchedule, len(rows))
	for i, row := range rows {
		schedules[i] = dbScheduleToModel(row)
	}
	return schedules, nil
}

func (r *PostgresFinanceRepository) MarkProRataApplied(ctx context.Context, scheduleID string) error {
	sid, err := pgutil.ParseUUID(scheduleID)
	if err != nil {
		return err
	}
	return r.queries.MarkProRataApplied(ctx, sid)
}

func (r *PostgresFinanceRepository) GetUpcomingProRata(ctx context.Context, userID string) ([]*model.ProRataSchedule, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.GetUpcomingProRata(ctx, uid)
	if err != nil {
		return nil, err
	}

	schedules := make([]*model.ProRataSchedule, len(rows))
	for i, row := range rows {
		schedules[i] = dbScheduleToModel(row)
	}
	return schedules, nil
}

// GetHealthScore reads the persisted closed-month score, returning nil when no
// row exists. The full model.HealthScore (components and insight) is the score
// JSONB column; the scalar columns are denormalized copies for trend reads.
func (r *PostgresFinanceRepository) GetHealthScore(ctx context.Context, userID string, year, month int32) (*model.HealthScore, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetHealthScore(ctx, db.GetHealthScoreParams{UserID: uid, Year: year, Month: month})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return dbHealthScoreToModel(row)
}

// UpsertHealthScore persists a closed-month score. The full score is stored as
// the score JSONB column (the single source of truth) and total/band/
// formula_version are denormalized into scalar columns for cheap trend reads.
func (r *PostgresFinanceRepository) UpsertHealthScore(ctx context.Context, userID string, score *model.HealthScore) (*model.HealthScore, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(score)
	if err != nil {
		return nil, fmt.Errorf("marshaling health score: %w", err)
	}

	row, err := r.queries.UpsertHealthScore(ctx, db.UpsertHealthScoreParams{
		UserID:         uid,
		Year:           score.Year,
		Month:          score.Month,
		Total:          score.Total,
		Band:           score.Band,
		Score:          payload,
		FormulaVersion: score.FormulaVersion,
	})
	if err != nil {
		return nil, err
	}
	return dbHealthScoreToModel(row)
}

func (r *PostgresFinanceRepository) DeleteAllUserData(ctx context.Context, userID string) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}

	// Delete in consistent order: pro_rata_schedules → tags → budget_periods → default_settings
	if err := r.queries.DeleteAllUserProRataSchedules(ctx, uid); err != nil {
		return fmt.Errorf("deleting pro_rata_schedules: %w", err)
	}
	if err := r.queries.DeleteAllUserTags(ctx, uid); err != nil {
		return fmt.Errorf("deleting tags: %w", err)
	}
	if err := r.queries.DeleteAllUserBudgetPeriods(ctx, uid); err != nil {
		return fmt.Errorf("deleting budget_periods: %w", err)
	}
	if err := r.queries.DeleteAllUserHealthScores(ctx, uid); err != nil {
		return fmt.Errorf("deleting health_scores: %w", err)
	}
	if err := r.queries.DeleteAllUserDefaultSettings(ctx, uid); err != nil {
		return fmt.Errorf("deleting default_settings: %w", err)
	}

	return nil
}

func dbDefaultsToModel(d db.FinanceDefaultSetting) *model.DefaultSettings {
	return &model.DefaultSettings{
		UserID:            uuid.UUID(d.UserID.Bytes).String(),
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
		ID:        uuid.UUID(t.ID.Bytes).String(),
		UserID:    uuid.UUID(t.UserID.Bytes).String(),
		Name:      t.Name,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt.Time,
		UpdatedAt: t.UpdatedAt.Time,
	}
}

func dbPeriodToModel(p db.FinanceBudgetPeriod) *model.BudgetPeriod {
	return &model.BudgetPeriod{
		ID:                uuid.UUID(p.ID.Bytes).String(),
		UserID:            uuid.UUID(p.UserID.Bytes).String(),
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

func dbHealthScoreToModel(h db.FinanceHealthScore) (*model.HealthScore, error) {
	var score model.HealthScore
	if err := json.Unmarshal(h.Score, &score); err != nil {
		return nil, fmt.Errorf("unmarshaling health score: %w", err)
	}
	return &score, nil
}

func dbScheduleToModel(s db.FinanceProRataSchedule) *model.ProRataSchedule {
	result := &model.ProRataSchedule{
		ID:               uuid.UUID(s.ID.Bytes).String(),
		UserID:           uuid.UUID(s.UserID.Bytes).String(),
		ProRataGroup:     uuid.UUID(s.ProRataGroup.Bytes).String(),
		Name:             s.Name,
		Amount:           s.Amount,
		Currency:         s.Currency,
		ExpenseType:      s.ExpenseType,
		TagID:            uuid.UUID(s.TagID.Bytes).String(),
		TargetYear:       s.TargetYear,
		TargetMonth:      s.TargetMonth,
		InstallmentIndex: s.InstallmentIndex,
		InstallmentTotal: s.InstallmentTotal,
		Status:           s.Status,
		CreatedAt:        s.CreatedAt.Time,
	}
	if s.AppliedAt.Valid {
		appliedAt := s.AppliedAt.Time
		result.AppliedAt = &appliedAt
	}
	return result
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
