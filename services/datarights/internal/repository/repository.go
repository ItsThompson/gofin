package repository

import "context"

import "github.com/ItsThompson/gofin/services/datarights/internal/model"

// JobRepository defines the data access contract for export job operations.
type JobRepository interface {
	CreateJob(ctx context.Context, userID string) (*model.ExportJob, error)
	GetJob(ctx context.Context, jobID string) (*model.ExportJob, error)
	ListJobsByUser(ctx context.Context, userID string, page, pageSize int) ([]*model.ExportJob, int64, error)
}
