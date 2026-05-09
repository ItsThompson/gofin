package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// ServiceError is a typed error that carries an HTTP status code and error code.
type ServiceError struct {
	Code    string
	Message string
	Status  int
}

func (e *ServiceError) Error() string {
	return e.Message
}

// ExportService handles export job lifecycle.
type ExportService struct {
	repo   repository.JobRepository
	logger *slog.Logger
}

// NewExportService creates a new ExportService.
func NewExportService(repo repository.JobRepository, logger *slog.Logger) *ExportService {
	return &ExportService{
		repo:   repo,
		logger: logger,
	}
}

// CreateJob creates a new pending export job for the user.
// In this walking skeleton, no engine submission occurs: the job stays in pending state.
func (s *ExportService) CreateJob(ctx context.Context, userID string) (*model.ExportJob, error) {
	s.logger.Info("creating export job",
		slog.String("user_id", userID),
	)

	job, err := s.repo.CreateJob(ctx, userID)
	if err != nil {
		s.logger.Error("failed to create export job",
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("creating job: %w", err)
	}

	s.logger.Info("export job created",
		slog.String("job_id", job.ID),
		slog.String("user_id", userID),
		slog.String("status", job.Status),
	)

	return job, nil
}

// GetJob retrieves a job by ID, scoped to the given user.
// Returns a 404 ServiceError if the job does not exist or belongs to another user.
func (s *ExportService) GetJob(ctx context.Context, jobID, userID string) (*model.ExportJob, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("getting job: %w", err)
	}

	if job == nil || job.UserID != userID {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: "Export job not found",
			Status:  404,
		}
	}

	return job, nil
}

// ListJobs returns a paginated list of jobs for the user.
func (s *ExportService) ListJobs(ctx context.Context, userID string, page, pageSize int) (*model.JobListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}

	jobs, total, err := s.repo.ListJobsByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	if jobs == nil {
		jobs = []*model.ExportJob{}
	}

	hasMore := int64(page*pageSize) < total

	return &model.JobListResponse{
		Data:     jobs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	}, nil
}
