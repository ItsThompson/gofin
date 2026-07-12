package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/pgutil"
)

// PostgresDeletionJobRepository implements DeletionJobRepository using PostgreSQL.
type PostgresDeletionJobRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresDeletionJobRepository creates a new PostgreSQL-backed deletion job repository.
func NewPostgresDeletionJobRepository(pool *pgxpool.Pool) *PostgresDeletionJobRepository {
	return &PostgresDeletionJobRepository{pool: pool}
}

// CreateJob inserts a new pending deletion job and returns the created record.
func (r *PostgresDeletionJobRepository) CreateJob(ctx context.Context, userID, adminUserID string) (*model.DeletionJob, error) {
	query := `
		INSERT INTO datarights.deletion_jobs (user_id, admin_user_id, status)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, admin_user_id, status, error, created_at, completed_at, updated_at`

	job := &model.DeletionJob{}
	err := r.pool.QueryRow(ctx, query, userID, adminUserID, model.StatusPending).Scan(
		&job.ID,
		&job.UserID,
		&job.AdminUserID,
		&job.Status,
		&job.Error,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating deletion job: %w", err)
	}

	return job, nil
}

// GetJob retrieves a single deletion job by ID.
func (r *PostgresDeletionJobRepository) GetJob(ctx context.Context, jobID string) (*model.DeletionJob, error) {
	query := `
		SELECT id, user_id, admin_user_id, status, error, created_at, completed_at, updated_at
		FROM datarights.deletion_jobs
		WHERE id = $1`

	job := &model.DeletionJob{}
	err := r.pool.QueryRow(ctx, query, jobID).Scan(
		&job.ID,
		&job.UserID,
		&job.AdminUserID,
		&job.Status,
		&job.Error,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting deletion job: %w", err)
	}

	return job, nil
}

// GetInProgressJob returns a pending or running deletion job for the user, if one exists.
func (r *PostgresDeletionJobRepository) GetInProgressJob(ctx context.Context, userID string) (*model.DeletionJob, error) {
	query := `
		SELECT id, user_id, admin_user_id, status, error, created_at, completed_at, updated_at
		FROM datarights.deletion_jobs
		WHERE user_id = $1 AND status IN ($2, $3)
		LIMIT 1`

	job := &model.DeletionJob{}
	err := r.pool.QueryRow(ctx, query, userID, model.StatusPending, model.StatusRunning).Scan(
		&job.ID,
		&job.UserID,
		&job.AdminUserID,
		&job.Status,
		&job.Error,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting in-progress deletion job: %w", err)
	}

	return job, nil
}

// UpdateStatus transitions a deletion job to the given status.
func (r *PostgresDeletionJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
	query := `
		UPDATE datarights.deletion_jobs
		SET status = $2, updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, status)
	if err != nil {
		return fmt.Errorf("updating deletion job status: %w", err)
	}

	return nil
}

// CompleteJob marks a deletion job as completed.
func (r *PostgresDeletionJobRepository) CompleteJob(ctx context.Context, jobID string) error {
	query := `
		UPDATE datarights.deletion_jobs
		SET status = $2, completed_at = now(), updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, model.StatusCompleted)
	if err != nil {
		return fmt.Errorf("completing deletion job: %w", err)
	}

	return nil
}

// FailJob marks a deletion job as failed with the given error message.
func (r *PostgresDeletionJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	query := `
		UPDATE datarights.deletion_jobs
		SET status = $2, error = $3, completed_at = now(), updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, model.StatusFailed, errMsg)
	if err != nil {
		return fmt.Errorf("failing deletion job: %w", err)
	}

	return nil
}

// GetNonTerminalJobs returns all deletion jobs in pending or running state for startup recovery.
func (r *PostgresDeletionJobRepository) GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableDeletionJob, error) {
	query := `
		SELECT id, user_id
		FROM datarights.deletion_jobs
		WHERE status IN ('pending', 'running')`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying non-terminal deletion jobs: %w", err)
	}
	defer rows.Close()

	var jobs []model.RecoverableDeletionJob
	for rows.Next() {
		var job model.RecoverableDeletionJob
		if err := rows.Scan(&job.ID, &job.UserID); err != nil {
			return nil, fmt.Errorf("scanning recoverable deletion job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recoverable deletion jobs: %w", err)
	}

	return jobs, nil
}
