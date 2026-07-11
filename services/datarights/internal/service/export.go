package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// RateLimitWindow is the minimum duration between successful exports for a user.
const RateLimitWindow = 30 * 24 * time.Hour

// RateLimitError is returned when the user has exceeded the export rate limit.
type RateLimitError struct {
	RetryAfter time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited until %s", e.RetryAfter.Format(time.RFC3339))
}

// CreateJobResult wraps the result of CreateJob, indicating whether
// an existing in-progress job was returned (dedup) or a new one was created.
type CreateJobResult struct {
	Job        *model.ExportJob
	IsExisting bool
}

// ExportService handles export job lifecycle.
type ExportService struct {
	repo          repository.JobRepository
	engine        *engine.Engine
	emailResolver UserEmailResolver
	logger        *slog.Logger
}

// NewExportService creates a new ExportService.
// The engine parameter may be nil (e.g., in tests that don't need engine submission).
func NewExportService(repo repository.JobRepository, logger *slog.Logger, opts ...ExportServiceOption) *ExportService {
	svc := &ExportService{
		repo:   repo,
		logger: logger,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// ExportServiceOption configures optional ExportService dependencies.
type ExportServiceOption func(*ExportService)

// WithEngine attaches an export engine to the service.
func WithEngine(eng *engine.Engine) ExportServiceOption {
	return func(s *ExportService) {
		s.engine = eng
	}
}

// WithEmailResolver attaches a user email resolver to the service.
func WithEmailResolver(resolver UserEmailResolver) ExportServiceOption {
	return func(s *ExportService) {
		s.emailResolver = resolver
	}
}

// CreateJob creates a new pending export job for the user.
// Returns an existing in-progress job if one exists (dedup).
// Returns a RateLimitError if the user exported within the rate limit window.
// Submits the new job to the engine for async processing.
func (s *ExportService) CreateJob(ctx context.Context, userID string) (*CreateJobResult, error) {
	// Dedup: check for an existing in-progress job
	existing, err := s.repo.GetInProgressJob(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking in-progress job: %w", err)
	}
	if existing != nil {
		s.logger.Info("returning existing in-progress job",
			slog.String("job_id", existing.ID),
			slog.String("user_id", userID),
		)
		return &CreateJobResult{Job: existing, IsExisting: true}, nil
	}

	// Rate limit: check for a recent non-failed job within the window
	latest, err := s.repo.GetLatestNonFailedJob(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking rate limit: %w", err)
	}
	if latest != nil {
		retryAfter := latest.CreatedAt.Add(RateLimitWindow)
		if time.Now().UTC().Before(retryAfter) {
			return nil, &RateLimitError{RetryAfter: retryAfter}
		}
	}

	// Create the new job
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

	// Submit to engine for async processing
	if s.engine != nil {
		userEmail, err := s.resolveUserEmail(ctx, userID)
		if err != nil {
			// Log but don't fail job creation: the engine will fail at the email step
			s.logger.Error("failed to resolve user email for export",
				slog.String("job_id", job.ID),
				slog.String("user_id", userID),
				slog.String("error", err.Error()),
			)
		}
		s.engine.Submit(job.ID, userID, userEmail)
	}

	return &CreateJobResult{Job: job, IsExisting: false}, nil
}

// GetJob retrieves a job by ID, scoped to the given user.
// Returns a 404 ServiceError if the job does not exist or belongs to another user.
func (s *ExportService) GetJob(ctx context.Context, jobID, userID string) (*model.ExportJob, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("getting job: %w", err)
	}

	if job == nil || job.UserID != userID {
		return nil, apierr.NotFound("Export job not found")
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

// resolveUserEmail fetches the user's email address via the configured resolver.
// Returns empty string if no resolver is configured or if resolution fails.
func (s *ExportService) resolveUserEmail(ctx context.Context, userID string) (string, error) {
	if s.emailResolver == nil {
		return "", fmt.Errorf("no email resolver configured")
	}
	return s.emailResolver.ResolveEmail(ctx, userID)
}
