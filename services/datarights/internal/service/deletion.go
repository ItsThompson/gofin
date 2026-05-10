package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// CreateDeletionJobResult wraps the result of CreateJob, indicating whether
// an existing in-progress job was returned (dedup) or a new one was created.
type CreateDeletionJobResult struct {
	Job        *model.DeletionJob
	IsExisting bool
}

// DeletionService handles deletion job lifecycle with full validation chain.
type DeletionService struct {
	repo       repository.DeletionJobRepository
	exportRepo repository.JobRepository
	authClient authpb.AuthServiceClient
	engine     DeletionJobSubmitter
	logger     *slog.Logger
}

// NewDeletionService creates a new DeletionService.
func NewDeletionService(
	repo repository.DeletionJobRepository,
	logger *slog.Logger,
	opts ...DeletionServiceOption,
) *DeletionService {
	svc := &DeletionService{
		repo:   repo,
		logger: logger,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// DeletionServiceOption configures optional DeletionService dependencies.
type DeletionServiceOption func(*DeletionService)

// WithAuthClient attaches the auth gRPC client.
func WithAuthClient(client authpb.AuthServiceClient) DeletionServiceOption {
	return func(s *DeletionService) {
		s.authClient = client
	}
}

// WithExportRepo attaches the export job repository for conflict checking.
func WithExportRepo(repo repository.JobRepository) DeletionServiceOption {
	return func(s *DeletionService) {
		s.exportRepo = repo
	}
}

// WithDeletionEngine attaches the deletion engine for job submission.
func WithDeletionEngine(engine DeletionJobSubmitter) DeletionServiceOption {
	return func(s *DeletionService) {
		s.engine = engine
	}
}

// CreateJob applies the full validation chain and creates a deletion job.
//
// Validation order:
//  1. Password verification via auth gRPC VerifyPassword
//  2. Self-deletion prevention (userId != adminUserID)
//  3. Protected username enforcement via auth gRPC GetUser
//  4. Export conflict detection (non-terminal export for target user)
//  5. Idempotent dedup (non-terminal deletion job already exists)
//  6. Create new job and submit to engine
func (s *DeletionService) CreateJob(ctx context.Context, userID, adminUserID, password string) (*CreateDeletionJobResult, error) {
	s.logger.Info("creating deletion job",
		slog.String("user_id", userID),
		slog.String("admin_user_id", adminUserID),
	)

	// Guard 1: Verify admin password
	if err := s.verifyPassword(ctx, adminUserID, password); err != nil {
		return nil, err
	}

	// Guard 2: Self-deletion prevention
	if userID == adminUserID {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "Cannot delete your own account",
			Status:  http.StatusBadRequest,
		}
	}

	// Guard 3: Protected username enforcement
	if err := s.checkProtectedUsername(ctx, userID); err != nil {
		return nil, err
	}

	// Guard 4: Export conflict detection
	if err := s.checkExportConflict(ctx, userID); err != nil {
		return nil, err
	}

	// Guard 5: Idempotent dedup
	existing, err := s.repo.GetInProgressJob(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking in-progress deletion job: %w", err)
	}
	if existing != nil {
		s.logger.Info("returning existing in-progress deletion job",
			slog.String("job_id", existing.ID),
			slog.String("user_id", userID),
		)
		return &CreateDeletionJobResult{Job: existing, IsExisting: true}, nil
	}

	// All guards passed: create the job
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

	// Submit to engine for async processing
	if s.engine != nil {
		s.engine.Submit(job.ID, userID)
	}

	return &CreateDeletionJobResult{Job: job, IsExisting: false}, nil
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

// verifyPassword checks the admin's password via auth gRPC VerifyPassword.
func (s *DeletionService) verifyPassword(ctx context.Context, adminUserID, password string) error {
	if s.authClient == nil {
		return fmt.Errorf("auth client not configured")
	}

	resp, err := s.authClient.VerifyPassword(ctx, &authpb.VerifyPasswordRequest{
		UserId:   adminUserID,
		Password: password,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return &ServiceError{
				Code:    model.ErrServiceUnavailable,
				Message: "Auth service unavailable",
				Status:  http.StatusServiceUnavailable,
			}
		}
		return fmt.Errorf("verifying password: %w", err)
	}

	if !resp.GetValid() {
		return &ServiceError{
			Code:    model.ErrInvalidCredentials,
			Message: "Invalid credentials",
			Status:  http.StatusUnauthorized,
		}
	}

	return nil
}

// checkProtectedUsername fetches the target user's username and rejects protected names.
func (s *DeletionService) checkProtectedUsername(ctx context.Context, userID string) error {
	if s.authClient == nil {
		return fmt.Errorf("auth client not configured")
	}

	resp, err := s.authClient.GetUser(ctx, &authpb.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return &ServiceError{
				Code:    model.ErrServiceUnavailable,
				Message: "Auth service unavailable",
				Status:  http.StatusServiceUnavailable,
			}
		}
		return fmt.Errorf("fetching user for protected check: %w", err)
	}

	if isProtectedUsername(resp.GetUsername()) {
		return &ServiceError{
			Code:    model.ErrProtectedUser,
			Message: "Cannot delete a protected user account",
			Status:  http.StatusForbidden,
		}
	}

	return nil
}

// checkExportConflict checks if the target user has an active export job.
func (s *DeletionService) checkExportConflict(ctx context.Context, userID string) error {
	if s.exportRepo == nil {
		return nil
	}

	exportJob, err := s.exportRepo.GetInProgressJob(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking export conflict: %w", err)
	}

	if exportJob != nil {
		return &ServiceError{
			Code:    model.ErrExportConflict,
			Message: "Cannot delete user while data export is in progress",
			Status:  http.StatusConflict,
		}
	}

	return nil
}
