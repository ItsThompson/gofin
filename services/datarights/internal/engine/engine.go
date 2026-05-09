package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
)

// Engine manages a bounded pool of export workers.
type Engine struct {
	registry *ProviderRegistry
	repo     repository.JobRepository
	sem      chan struct{}
	logger   *slog.Logger
	timeout  time.Duration
}

// NewEngine creates an export engine with bounded concurrency.
func NewEngine(
	registry *ProviderRegistry,
	repo repository.JobRepository,
	maxConcurrent int,
	timeout time.Duration,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		registry: registry,
		repo:     repo,
		sem:      make(chan struct{}, maxConcurrent),
		logger:   logger,
		timeout:  timeout,
	}
}

// Submit enqueues a job for asynchronous processing.
func (e *Engine) Submit(jobID, userID, userEmail string) {
	go func() {
		// Acquire semaphore slot (blocks if pool is full)
		e.sem <- struct{}{}
		defer func() { <-e.sem }()

		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		defer cancel()

		e.execute(ctx, jobID, userID, userEmail)
	}()
}

// execute runs the full export flow for a single job.
func (e *Engine) execute(ctx context.Context, jobID, userID, userEmail string) {
	e.logger.Info("export job starting",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
	)

	// Transition to running
	if err := e.repo.UpdateStatus(ctx, jobID, "running"); err != nil {
		e.logger.Error("failed to update job status to running",
			slog.String("job_id", jobID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Collect data from all providers
	var csvFiles []CSVFile
	for _, provider := range e.registry.All() {
		// Check context before each provider
		if err := ctx.Err(); err != nil {
			e.failJob(ctx, jobID, "Export timed out")
			return
		}

		rows, err := provider.Collect(ctx, userID)
		if err != nil {
			if ctx.Err() != nil {
				e.failJob(ctx, jobID, "Export timed out")
				return
			}
			e.failJob(ctx, jobID, fmt.Sprintf("Failed to collect %s data", provider.Name()))
			return
		}

		csvFiles = append(csvFiles, CSVFile{
			Name:    provider.Name() + ".csv",
			Headers: provider.Headers(),
			Rows:    rows,
		})
	}

	// Build ZIP
	zipBytes, err := BuildZIP(csvFiles)
	if err != nil {
		e.failJob(ctx, jobID, "Failed to build export archive")
		return
	}

	// Mark complete (email delivery is added in ticket #6)
	if err := e.repo.CompleteJob(ctx, jobID, int64(len(zipBytes))); err != nil {
		e.logger.Error("failed to complete job",
			slog.String("job_id", jobID),
			slog.String("error", err.Error()),
		)
		return
	}

	e.logger.Info("export job completed",
		slog.String("job_id", jobID),
		slog.Int64("file_size_bytes", int64(len(zipBytes))),
	)
}

// failJob marks a job as failed with a human-readable error message.
func (e *Engine) failJob(ctx context.Context, jobID, errMsg string) {
	e.logger.Warn("export job failed",
		slog.String("job_id", jobID),
		slog.String("error", errMsg),
	)

	// Use a background context for the fail update in case the original context expired
	failCtx := context.Background()
	if err := e.repo.FailJob(failCtx, jobID, errMsg); err != nil {
		e.logger.Error("failed to mark job as failed",
			slog.String("job_id", jobID),
			slog.String("error", err.Error()),
		)
	}
}
