package deletion

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/datarights/internal/jobrunner"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

const (
	maxRetries    = 3
	backoffFirst  = 1 * time.Second
	backoffSecond = 2 * time.Second
)

// Engine manages a bounded pool of deletion workers. It shares the pool
// lifecycle with the export engine via jobrunner.Pool and injects its own
// execution strategy: each provider is run sequentially in registration order
// with per-provider retry and backoff. The deletion job repo satisfies
// jobrunner.StatusStore directly, so it is handed to the pool as the store.
type Engine struct {
	registry *Registry
	pool     *jobrunner.Pool
	logger   *slog.Logger
}

// NewEngine creates a deletion engine with bounded concurrency.
func NewEngine(
	registry *Registry,
	repo repository.DeletionJobRepository,
	maxConcurrent int,
	timeout time.Duration,
	logger *slog.Logger,
) *Engine {
	e := &Engine{
		registry: registry,
		logger:   logger,
	}
	e.pool = jobrunner.New(maxConcurrent, timeout, repo, e.execute, logger)
	return e
}

// Submit enqueues a deletion job for async processing. Non-blocking: the pool
// spawns a goroutine that blocks on the semaphore.
func (e *Engine) Submit(jobID, userID string) {
	e.pool.Submit(jobID, userID)
}

// execute is the injected jobrunner strategy: it runs every provider in
// registration order and returns the first exhausted-retry (or context) error.
// The pool owns the running transition, completion, and failure persistence.
func (e *Engine) execute(ctx context.Context, jobID, userID string) error {
	e.logger.Info("deletion job starting",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "deletion.engine.execute"),
	)

	for _, provider := range e.registry.All() {
		attempts, err := e.executeProvider(ctx, provider, jobID, userID)
		if err != nil {
			// Provider exhausted retries or context expired: fail the job.
			jobErr := fmt.Errorf("provider %s failed after %d attempts: %w", provider.Name(), attempts, err)
			e.logger.Error("deletion job failed",
				slog.String("job_id", jobID),
				slog.String("user_id", userID),
				slog.String("error", jobErr.Error()),
				slog.String("method", "deletion.engine.execute"),
			)
			return jobErr
		}
	}

	e.logger.Info("deletion job completed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "deletion.engine.execute"),
	)
	return nil
}

// executeProvider attempts a single provider up to maxRetries times with backoff.
// Returns the number of attempts made and nil if the provider eventually succeeds,
// or the number of attempts and the last error if retries are exhausted or context expires.
func (e *Engine) executeProvider(ctx context.Context, provider Provider, jobID, userID string) (int, error) {
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
			return attempt, nil
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
				return attempt, ctx.Err()
			}
		}
	}

	return maxRetries, lastErr
}
