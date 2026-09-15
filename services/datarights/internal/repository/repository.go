package repository

import (
	"context"
	"time"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

// ExportMetricsWindowSet contains the UTC ranges used by the export metric.
type ExportMetricsWindowSet struct {
	ReportWeekStart        time.Time
	ReportWeekEnd          time.Time
	PreviousWeekStart      time.Time
	PreviousWeekEnd        time.Time
	TrailingFourWeeksStart time.Time
	TrailingFourWeeksEnd   time.Time
}

// CompletedExportCounts contains one count for each reporting range.
type CompletedExportCounts struct {
	ReportWeek             int64
	PreviousWeek           int64
	TrailingFourWeeksTotal int64
}

// ExportMetricsRepository defines the data access contract for export metrics.
type ExportMetricsRepository interface {
	CountCompletedExports(ctx context.Context, windows ExportMetricsWindowSet) (CompletedExportCounts, error)
}

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
