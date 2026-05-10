package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// DeletionService handles deletion job lifecycle.
type DeletionService struct {
	repo   repository.DeletionJobRepository
	logger *slog.Logger
}

// NewDeletionService creates a new DeletionService.
func NewDeletionService(repo repository.DeletionJobRepository, logger *slog.Logger) *DeletionService {
	return &DeletionService{
		repo:   repo,
		logger: logger,
	}
}

// CreateJob creates a new pending deletion job for the target user.
// The adminUserID is extracted from the request header by the handler.
// Password validation is NOT performed here (deferred to a later ticket).
func (s *DeletionService) CreateJob(ctx context.Context, userID, adminUserID string) (*model.DeletionJob, error) {
	s.logger.Info("creating deletion job",
		slog.String("user_id", userID),
		slog.String("admin_user_id", adminUserID),
	)

	job, err := s.repo.CreateJob(ctx, userID, adminUserID)
	if err != nil {
		s.logger.Error("failed to create deletion job",
			slog.String("user_id", userID),
			slog.String("admin_user_id", adminUserID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("creating deletion job: %w", err)
	}

	s.logger.Info("deletion job created",
		slog.String("job_id", job.ID),
		slog.String("user_id", userID),
		slog.String("admin_user_id", adminUserID),
		slog.String("status", job.Status),
	)

	return job, nil
}

// GetJob retrieves a deletion job by ID.
// Returns a 404 ServiceError if the job does not exist.
func (s *DeletionService) GetJob(ctx context.Context, jobID string) (*model.DeletionJob, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("getting deletion job: %w", err)
	}

	if job == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: "Deletion job not found",
			Status:  http.StatusNotFound,
		}
	}

	return job, nil
}
