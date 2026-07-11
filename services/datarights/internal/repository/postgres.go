package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/pgutil"
)

// PostgresJobRepository implements JobRepository using PostgreSQL.
type PostgresJobRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresJobRepository creates a new PostgreSQL-backed job repository.
func NewPostgresJobRepository(pool *pgxpool.Pool) *PostgresJobRepository {
	return &PostgresJobRepository{pool: pool}
}

// CreateJob inserts a new pending export job and returns the created record.
func (r *PostgresJobRepository) CreateJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	query := `
		INSERT INTO datarights.export_jobs (user_id, status)
		VALUES ($1, $2)
		RETURNING id, user_id, status, error, file_size_bytes, created_at, completed_at, updated_at`

	job := &model.ExportJob{}
	err := r.pool.QueryRow(ctx, query, userID, model.StatusPending).Scan(
		&job.ID,
		&job.UserID,
		&job.Status,
		&job.Error,
		&job.FileSizeBytes,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating export job: %w", err)
	}

	return job, nil
}

// GetJob retrieves a single export job by ID.
func (r *PostgresJobRepository) GetJob(ctx context.Context, jobID string) (*model.ExportJob, error) {
	query := `
		SELECT id, user_id, status, error, file_size_bytes, created_at, completed_at, updated_at
		FROM datarights.export_jobs
		WHERE id = $1`

	job := &model.ExportJob{}
	err := r.pool.QueryRow(ctx, query, jobID).Scan(
		&job.ID,
		&job.UserID,
		&job.Status,
		&job.Error,
		&job.FileSizeBytes,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting export job: %w", err)
	}

	return job, nil
}

// GetInProgressJob returns a pending or running job for the user, if one exists.
func (r *PostgresJobRepository) GetInProgressJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	query := `
		SELECT id, user_id, status, error, file_size_bytes, created_at, completed_at, updated_at
		FROM datarights.export_jobs
		WHERE user_id = $1 AND status IN ($2, $3)
		LIMIT 1`

	job := &model.ExportJob{}
	err := r.pool.QueryRow(ctx, query, userID, model.StatusPending, model.StatusRunning).Scan(
		&job.ID,
		&job.UserID,
		&job.Status,
		&job.Error,
		&job.FileSizeBytes,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting in-progress job: %w", err)
	}

	return job, nil
}

// GetLatestNonFailedJob returns the most recently created non-failed job for the user.
// Used for rate limit enforcement: failed jobs don't count toward the 30-day cooldown.
func (r *PostgresJobRepository) GetLatestNonFailedJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	query := `
		SELECT id, user_id, status, error, file_size_bytes, created_at, completed_at, updated_at
		FROM datarights.export_jobs
		WHERE user_id = $1 AND status != $2
		ORDER BY created_at DESC
		LIMIT 1`

	job := &model.ExportJob{}
	err := r.pool.QueryRow(ctx, query, userID, model.StatusFailed).Scan(
		&job.ID,
		&job.UserID,
		&job.Status,
		&job.Error,
		&job.FileSizeBytes,
		&job.CreatedAt,
		&job.CompletedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if pgutil.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting latest non-failed job: %w", err)
	}

	return job, nil
}

// ListJobsByUser returns a paginated list of export jobs for a user,
// ordered by creation date descending (newest first).
func (r *PostgresJobRepository) ListJobsByUser(ctx context.Context, userID string, page, pageSize int) ([]*model.ExportJob, int64, error) {
	countQuery := `SELECT COUNT(*) FROM datarights.export_jobs WHERE user_id = $1`

	var total int64
	err := r.pool.QueryRow(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting export jobs: %w", err)
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, user_id, status, error, file_size_bytes, created_at, completed_at, updated_at
		FROM datarights.export_jobs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing export jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*model.ExportJob
	for rows.Next() {
		job := &model.ExportJob{}
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.Status,
			&job.Error,
			&job.FileSizeBytes,
			&job.CreatedAt,
			&job.CompletedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning export job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating export jobs: %w", err)
	}

	return jobs, total, nil
}

// UpdateStatus transitions a job to the given status.
func (r *PostgresJobRepository) UpdateStatus(ctx context.Context, jobID string, status string) error {
	query := `
		UPDATE datarights.export_jobs
		SET status = $2, updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, status)
	if err != nil {
		return fmt.Errorf("updating job status: %w", err)
	}

	return nil
}

// CompleteJob marks a job as completed with the given file size.
func (r *PostgresJobRepository) CompleteJob(ctx context.Context, jobID string, fileSizeBytes int64) error {
	query := `
		UPDATE datarights.export_jobs
		SET status = $2, file_size_bytes = $3, completed_at = now(), updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, model.StatusCompleted, fileSizeBytes)
	if err != nil {
		return fmt.Errorf("completing job: %w", err)
	}

	return nil
}

// FailJob marks a job as failed with the given error message.
func (r *PostgresJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	query := `
		UPDATE datarights.export_jobs
		SET status = $2, error = $3, completed_at = now(), updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, jobID, model.StatusFailed, errMsg)
	if err != nil {
		return fmt.Errorf("failing job: %w", err)
	}

	return nil
}

// GetNonTerminalJobs returns all jobs in pending or running state for startup recovery.
func (r *PostgresJobRepository) GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableJob, error) {
	query := `
		SELECT id, user_id
		FROM datarights.export_jobs
		WHERE status IN ('pending', 'running')`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying non-terminal jobs: %w", err)
	}
	defer rows.Close()

	var jobs []model.RecoverableJob
	for rows.Next() {
		var job model.RecoverableJob
		if err := rows.Scan(&job.ID, &job.UserID); err != nil {
			return nil, fmt.Errorf("scanning recoverable job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recoverable jobs: %w", err)
	}

	return jobs, nil
}
