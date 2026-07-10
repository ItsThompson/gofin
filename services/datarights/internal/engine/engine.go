package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// ProviderFactory builds a fresh set of data providers for a single export job,
// injecting the finance client the finance-backed providers should share. The
// factory closes over the non-finance clients (auth, expense) at startup; the
// finance client is supplied per job so each job gets its own memoized instance.
type ProviderFactory func(finance financepb.FinanceServiceClient) []DataProvider

// Engine manages a bounded pool of export workers.
type Engine struct {
	newProviders  ProviderFactory
	financeClient financepb.FinanceServiceClient
	repo          repository.JobRepository
	sender        email.Sender
	sem           chan struct{}
	logger        *slog.Logger
	timeout       time.Duration
}

// NewEngine creates an export engine with bounded concurrency. newProviders
// builds a fresh provider set per job; financeClient is the raw finance client
// wrapped in a per-job MemoizedFinanceClient before being handed to the factory.
func NewEngine(
	newProviders ProviderFactory,
	financeClient financepb.FinanceServiceClient,
	repo repository.JobRepository,
	sender email.Sender,
	maxConcurrent int,
	timeout time.Duration,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		newProviders:  newProviders,
		financeClient: financeClient,
		repo:          repo,
		sender:        sender,
		sem:           make(chan struct{}, maxConcurrent),
		logger:        logger,
		timeout:       timeout,
	}
}

// ActiveJobs returns the number of currently executing export jobs.
func (e *Engine) ActiveJobs() int {
	return len(e.sem)
}

// MaxConcurrent returns the pool capacity.
func (e *Engine) MaxConcurrent() int {
	return cap(e.sem)
}

// Submit enqueues a job for asynchronous processing.
func (e *Engine) Submit(jobID, userID, userEmail string) {
	go func() {
		// Track queued state: job is waiting for a pool slot
		exportmetrics.ExportPoolQueuedJobs.Inc()

		// Acquire semaphore slot (blocks if pool is full)
		e.sem <- struct{}{}

		// No longer queued, now active
		exportmetrics.ExportPoolQueuedJobs.Dec()
		exportmetrics.ExportPoolActiveJobs.Inc()

		defer func() {
			<-e.sem
			exportmetrics.ExportPoolActiveJobs.Dec()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		defer cancel()

		e.execute(ctx, jobID, userID, userEmail)
	}()
}

// execute runs the full export flow for a single job.
func (e *Engine) execute(ctx context.Context, jobID, userID, userEmail string) {
	jobStart := time.Now()

	e.logger.Info("export job starting",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "engine.execute"),
	)

	// Transition to running
	if err := e.repo.UpdateStatus(ctx, jobID, "running"); err != nil {
		e.logger.Error("failed to update job status to running",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "engine.execute"),
			slog.String("error", err.Error()),
		)
		exportmetrics.ExportJobsCompletedTotal.WithLabelValues("failed").Inc()
		exportmetrics.ExportJobDurationSeconds.Observe(time.Since(jobStart).Seconds())
		return
	}

	// Build a fresh provider set for this job. The finance-backed providers
	// share one per-job MemoizedFinanceClient, so GetAllUserData is fetched at
	// most once; a fresh instance per job prevents cross-user data leakage.
	fc := NewMemoizedFinanceClient(e.financeClient)
	providerSet := e.newProviders(fc)

	// Collect from every provider concurrently. The providers are independent
	// and read-only, so each goroutine writes only its own pre-assigned index in
	// csvFiles; this makes collection latency max(providers) instead of sum while
	// keeping ZIP order deterministic (the factory's slice order).
	csvFiles := make([]CSVFile, len(providerSet))
	g, gctx := errgroup.WithContext(ctx)
	for i, provider := range providerSet {
		i, provider := i, provider
		g.Go(func() error {
			e.logger.Debug("provider collection started",
				slog.String("job_id", jobID),
				slog.String("user_id", userID),
				slog.String("provider", provider.Name()),
				slog.String("method", "engine.execute"),
			)

			providerStart := time.Now()
			rows, err := provider.Collect(gctx, userID)
			providerDuration := time.Since(providerStart).Seconds()

			// HistogramVec is goroutine-safe, so the per-provider observation
			// stays inside the goroutine.
			exportmetrics.ExportDataCollectionDurationSeconds.WithLabelValues(provider.Name()).Observe(providerDuration)

			if err != nil {
				// Bake the provider name into the error before errgroup captures
				// it: Wait surfaces only the first error, and humanCollectMessage
				// relies on this "collect <name>: ..." shape.
				return fmt.Errorf("collect %s: %w", provider.Name(), err)
			}

			e.logger.Info("provider collection complete",
				slog.String("job_id", jobID),
				slog.String("user_id", userID),
				slog.String("provider", provider.Name()),
				slog.Int("row_count", len(rows)),
				slog.Float64("duration_seconds", providerDuration),
				slog.String("method", "engine.execute"),
			)

			// Write only this goroutine's own slot; never append (data race).
			csvFiles[i] = CSVFile{
				Name:    provider.Name() + ".csv",
				Headers: provider.Headers(),
				Rows:    rows,
			}
			return nil
		})
	}

	// Fan-in barrier: the first error wins and cancels the siblings via gctx.
	// Recheck the job context afterward so a deadline still maps to "Export timed
	// out" while a genuine provider failure maps to "Failed to collect X data".
	switch err := g.Wait(); {
	case err == nil:
		// all providers succeeded; fall through to ZIP assembly
	case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
		e.failJob(ctx, jobID, userID, "Export timed out", "collection", jobStart)
		return
	default:
		e.failJob(ctx, jobID, userID, humanCollectMessage(err), "collection", jobStart)
		return
	}

	// Build ZIP
	zipBytes, err := BuildZIP(csvFiles)
	if err != nil {
		e.failJob(ctx, jobID, userID, "Failed to build export archive", "zip_assembly", jobStart)
		return
	}

	fileSizeBytes := int64(len(zipBytes))
	exportmetrics.ExportZipSizeBytes.Observe(float64(fileSizeBytes))

	e.logger.Info("ZIP assembled",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.Int64("file_size_bytes", fileSizeBytes),
		slog.String("method", "engine.execute"),
	)

	// Send email with ZIP attachment
	emailStart := time.Now()
	if err := e.sender.SendExportEmail(ctx, userEmail, zipBytes); err != nil {
		e.failJob(ctx, jobID, userID, fmt.Sprintf("Email delivery failed: %s", sanitizeError(err)), "email_delivery", jobStart)
		return
	}
	exportmetrics.ExportEmailSendDurationSeconds.Observe(time.Since(emailStart).Seconds())

	e.logger.Info("email sent",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.Float64("duration_seconds", time.Since(emailStart).Seconds()),
		slog.String("method", "engine.execute"),
	)

	// Mark complete
	if err := e.repo.CompleteJob(ctx, jobID, fileSizeBytes); err != nil {
		e.logger.Error("failed to complete job",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "engine.execute"),
			slog.String("error", err.Error()),
		)
		exportmetrics.ExportJobsCompletedTotal.WithLabelValues("failed").Inc()
		exportmetrics.ExportJobDurationSeconds.Observe(time.Since(jobStart).Seconds())
		return
	}

	jobDuration := time.Since(jobStart).Seconds()
	exportmetrics.ExportJobsCompletedTotal.WithLabelValues("completed").Inc()
	exportmetrics.ExportJobDurationSeconds.Observe(jobDuration)

	e.logger.Info("export job completed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.Int64("file_size_bytes", fileSizeBytes),
		slog.Float64("duration_seconds", jobDuration),
		slog.String("method", "engine.execute"),
	)
}

// failJob marks a job as failed with a human-readable error message.
func (e *Engine) failJob(_ context.Context, jobID, userID, errMsg, stage string, jobStart time.Time) {
	exportmetrics.ExportJobsCompletedTotal.WithLabelValues("failed").Inc()
	exportmetrics.ExportJobDurationSeconds.Observe(time.Since(jobStart).Seconds())

	e.logger.Error("export job failed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("error", errMsg),
		slog.String("stage", stage),
		slog.String("method", "engine.failJob"),
	)

	// Use a background context for the fail update in case the original context expired
	failCtx := context.Background()
	if err := e.repo.FailJob(failCtx, jobID, errMsg); err != nil {
		e.logger.Error("failed to mark job as failed",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("method", "engine.failJob"),
			slog.String("error", err.Error()),
		)
	}
}

// humanCollectMessage maps a wrapped provider-collection error, which the
// fan-out shapes as "collect <name>: <cause>", to the user-facing
// "Failed to collect <name> data" message without exposing the underlying
// cause. Provider names never contain a colon, so the first one delimits the
// name.
func humanCollectMessage(err error) string {
	name := "export"
	if rest, ok := strings.CutPrefix(err.Error(), "collect "); ok {
		if idx := strings.IndexByte(rest, ':'); idx != -1 {
			name = rest[:idx]
		}
	}
	return fmt.Sprintf("Failed to collect %s data", name)
}

// sanitizeError extracts a human-readable reason from an error without exposing internals.
func sanitizeError(err error) string {
	msg := err.Error()
	// Strip wrapping context to get the last meaningful message
	if idx := strings.LastIndex(msg, ": "); idx != -1 {
		msg = msg[idx+2:]
	}
	// Cap length to avoid overly long error messages
	if len(msg) > 100 {
		msg = msg[:100]
	}
	return msg
}
