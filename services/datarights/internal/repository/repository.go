package repository

import (
	"context"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

// JobRepository defines the data access contract for export job operations.
type JobRepository interface {
	CreateJob(ctx context.Context, userID string) (*model.ExportJob, error)
	GetJob(ctx context.Context, jobID string) (*model.ExportJob, error)
	ListJobsByUser(ctx context.Context, userID string, page, pageSize int) ([]*model.ExportJob, int64, error)
	GetInProgressJob(ctx context.Context, userID string) (*model.ExportJob, error)
	GetLatestNonFailedJob(ctx context.Context, userID string) (*model.ExportJob, error)
	UpdateStatus(ctx context.Context, jobID string, status string) error
	CompleteJob(ctx context.Context, jobID string, fileSizeBytes int64) error
	FailJob(ctx context.Context, jobID string, errMsg string) error
	GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableJob, error)
}

// DeletionJobRepository defines the data access contract for deletion job operations.
type DeletionJobRepository interface {
	CreateJob(ctx context.Context, userID, adminUserID string) (*model.DeletionJob, error)
	GetJob(ctx context.Context, jobID string) (*model.DeletionJob, error)
	GetInProgressJob(ctx context.Context, userID string) (*model.DeletionJob, error)
	UpdateStatus(ctx context.Context, jobID string, status string) error
	CompleteJob(ctx context.Context, jobID string) error
	FailJob(ctx context.Context, jobID string, errMsg string) error
	GetNonTerminalJobs(ctx context.Context) ([]model.RecoverableDeletionJob, error)
}
