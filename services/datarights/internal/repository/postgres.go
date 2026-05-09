package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting export job: %w", err)
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
