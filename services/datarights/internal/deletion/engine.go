package deletion

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

const (
	maxRetries    = 3
	backoffFirst  = 1 * time.Second
	backoffSecond = 2 * time.Second
)

// DeletionEngine manages a bounded pool of deletion workers.
// It is structurally similar to the export Engine but with different
// execution semantics (retry per provider, no email/zip step).
type DeletionEngine struct {
	registry *DeletionProviderRegistry
	repo     repository.DeletionJobRepository
	sem      chan struct{}
	logger   *slog.Logger
	timeout  time.Duration
}

// NewDeletionEngine creates a deletion engine with bounded concurrency.
func NewDeletionEngine(
	registry *DeletionProviderRegistry,
	repo repository.DeletionJobRepository,
	maxConcurrent int,
	timeout time.Duration,
	logger *slog.Logger,
) *DeletionEngine {
	return &DeletionEngine{
		registry: registry,
		repo:     repo,
		sem:      make(chan struct{}, maxConcurrent),
		logger:   logger,
		timeout:  timeout,
	}
}

// ActiveJobs returns the number of currently executing deletion jobs.
func (e *DeletionEngine) ActiveJobs() int {
	return len(e.sem)
}

// MaxConcurrent returns the pool capacity.
func (e *DeletionEngine) MaxConcurrent() int {
	return cap(e.sem)
}

// Submit enqueues a deletion job for async processing.
// Non-blocking: spawns a goroutine that blocks on the semaphore.
func (e *DeletionEngine) Submit(jobID, userID string) {
	go func() {
		// Acquire semaphore slot (blocks if pool is full)
		e.sem <- struct{}{}
		defer func() { <-e.sem }()

		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		defer cancel()

		e.execute(ctx, jobID, userID)
	}()
}

// execute runs the full deletion flow for a single job.
// Called inside a goroutine after acquiring a semaphore slot.
func (e *DeletionEngine) execute(ctx context.Context, jobID, userID string) {
	e.logger.Info("deletion job starting",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "deletion.engine.execute"),
	)

	// Transition job to "running"
	if err := e.repo.UpdateStatus(ctx, jobID, "running"); err != nil {
		e.logger.Error("failed to update deletion job status to running",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "deletion.engine.execute"),
			slog.String("error", err.Error()),
		)
		return
	}

	// Execute each provider in registration order
	for _, provider := range e.registry.All() {
		if err := e.executeProvider(ctx, provider, jobID, userID); err != nil {
			// Provider exhausted retries: mark job as failed
			errMsg := fmt.Sprintf("provider %s failed after %d attempts: %s", provider.Name(), maxRetries, err.Error())
			e.failJob(jobID, userID, errMsg)
			return
		}
	}

	// All providers succeeded: mark job as completed
	if err := e.repo.CompleteJob(ctx, jobID); err != nil {
		e.logger.Error("failed to complete deletion job",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "deletion.engine.execute"),
			slog.String("error", err.Error()),
		)
		return
	}

	e.logger.Info("deletion job completed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "deletion.engine.execute"),
	)
}

// executeProvider attempts a single provider up to maxRetries times with backoff.
// Returns nil if the provider eventually succeeds, or the last error if retries are exhausted.
func (e *DeletionEngine) executeProvider(ctx context.Context, provider DeletionProvider, jobID, userID string) error {
	backoffs := []time.Duration{backoffFirst, backoffSecond}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		e.logger.Debug("executing deletion provider",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("provider", provider.Name()),
			slog.Int("attempt", attempt),
			slog.String("method", "deletion.engine.executeProvider"),
		)

		err := provider.Delete(ctx, userID)
		if err == nil {
			if attempt > 1 {
				e.logger.Info("deletion provider succeeded on retry",
					slog.String("job_id", jobID),
					slog.String("user_id", userID),
					slog.String("provider", provider.Name()),
					slog.Int("attempt", attempt),
					slog.String("method", "deletion.engine.executeProvider"),
				)
			}
			return nil
		}

		lastErr = err
		e.logger.Warn("deletion provider attempt failed",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("provider", provider.Name()),
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
			slog.String("method", "deletion.engine.executeProvider"),
		)

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			backoff := backoffs[attempt-1]
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return lastErr
}

// failJob marks a job as failed using a background context (in case the original expired).
func (e *DeletionEngine) failJob(jobID, userID, errMsg string) {
	e.logger.Error("deletion job failed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("error", errMsg),
		slog.String("method", "deletion.engine.failJob"),
	)

	failCtx := context.Background()
	if err := e.repo.FailJob(failCtx, jobID, errMsg); err != nil {
		e.logger.Error("failed to mark deletion job as failed",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "deletion.engine.failJob"),
			slog.String("error", err.Error()),
		)
	}
}
