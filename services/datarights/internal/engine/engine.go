package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/jobrunner"
	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/serverkit"
)

// ProviderFactory builds a fresh set of data providers for a single export job
// from the finance data fetched once upfront. It closes over the self-fetching
// clients (auth for the profile, expense for the expenses stream) at startup;
// the resolved finance response is supplied per job so the finance-backed
// providers are pure response -> rows mappers and the export hits finance
// exactly once (in execute).
type ProviderFactory func(financeData *financepb.AllUserDataResponse) []DataProvider

// jobState carries the per-job values the pool's fixed Execute/StatusStore
// signatures cannot thread through: userEmail flows in (set by Submit, read by
// the execute strategy) and fileSize flows out (set by execute, read by the
// StatusStore adapter's CompleteJob). One job runs per jobID, and the pool
// drives that job's transitions sequentially in a single goroutine, so a plain
// pointer needs no further locking; sync.Map guards concurrent jobs.
type jobState struct {
	userEmail string
	fileSize  int64
}

// Engine manages a bounded pool of export workers via jobrunner.Pool, injecting
// its errgroup fan-out + zip + email closure as the Execute strategy. Because
// export's CompleteJob persists file_size_bytes (and Submit carries userEmail),
// neither of which fit the generic pool signatures, the engine adapts its repo
// behind the StatusStore seam (see statusStore) rather than handing the repo to
// the pool directly.
type Engine struct {
	newProviders  ProviderFactory
	financeClient financepb.FinanceServiceClient
	repo          repository.JobRepository
	sender        email.Sender
	logger        *slog.Logger
	pool          *jobrunner.Pool
	jobs          sync.Map // jobID -> *jobState
}

// NewEngine creates an export engine with bounded concurrency. newProviders
// builds a fresh provider set per job from the finance response that execute
// fetches once via financeClient.
func NewEngine(
	newProviders ProviderFactory,
	financeClient financepb.FinanceServiceClient,
	repo repository.JobRepository,
	sender email.Sender,
	maxConcurrent int,
	timeout time.Duration,
	logger *slog.Logger,
) *Engine {
	e := &Engine{
		newProviders:  newProviders,
		financeClient: financeClient,
		repo:          repo,
		sender:        sender,
		logger:        logger,
	}
	e.pool = jobrunner.New(maxConcurrent, timeout, statusStore{e}, e.execute, logger)
	return e
}

// ActiveJobs returns the number of currently executing export jobs.
func (e *Engine) ActiveJobs() int { return e.pool.ActiveJobs() }

// QueuedJobs returns the number of export jobs waiting for a pool slot.
func (e *Engine) QueuedJobs() int { return e.pool.QueuedJobs() }

// MaxConcurrent returns the pool capacity.
func (e *Engine) MaxConcurrent() int { return e.pool.MaxConcurrent() }

// Submit enqueues a job for asynchronous processing. Non-blocking: the pool
// spawns the worker goroutine.
func (e *Engine) Submit(jobID, userID, userEmail string) {
	e.jobs.Store(jobID, &jobState{userEmail: userEmail})
	e.pool.Submit(jobID, userID)
}

func (e *Engine) loadState(jobID string) *jobState {
	if v, ok := e.jobs.Load(jobID); ok {
		return v.(*jobState)
	}
	return &jobState{}
}

func (e *Engine) takeState(jobID string) *jobState {
	if v, ok := e.jobs.LoadAndDelete(jobID); ok {
		return v.(*jobState)
	}
	return &jobState{}
}

// statusStore adapts the export engine's repo to jobrunner.StatusStore. The
// pool's CompleteJob carries no size, so it reads the archive size the execute
// strategy recorded for the job; export's CompleteJob still persists
// file_size_bytes. The deletion repo satisfies StatusStore directly.
type statusStore struct{ e *Engine }

func (s statusStore) UpdateStatus(ctx context.Context, jobID, status string) error {
	if err := s.e.repo.UpdateStatus(ctx, jobID, status); err != nil {
		// The running transition failed, so the pool will not run execute or a
		// terminal transition: drop the per-job state now so it never leaks.
		s.e.jobs.Delete(jobID)
		return err
	}
	return nil
}

func (s statusStore) CompleteJob(ctx context.Context, jobID string) error {
	return s.e.repo.CompleteJob(ctx, jobID, s.e.takeState(jobID).fileSize)
}

func (s statusStore) FailJob(ctx context.Context, jobID, reason string) error {
	s.e.jobs.Delete(jobID)
	return s.e.repo.FailJob(ctx, jobID, reason)
}

// execute is the injected jobrunner strategy. It reads the per-job userEmail
// stashed by Submit, runs the export work, and records the archive size for the
// pool's CompleteJob. The pool owns the running transition, completion, and
// background-context failure.
func (e *Engine) execute(ctx context.Context, jobID, userID string) error {
	st := e.loadState(jobID)
	fileSizeBytes, err := e.runExport(ctx, jobID, userID, st.userEmail)
	if err != nil {
		return err
	}
	st.fileSize = fileSizeBytes
	return nil
}

// runExport performs the full export work for a single job: fan-out collection,
// ZIP assembly, and email delivery. It returns the archive size on success or a
// PII-free failure whose message is persisted as the job's error.
func (e *Engine) runExport(ctx context.Context, jobID, userID, userEmail string) (int64, error) {
	jobStart := time.Now()

	e.logger.Info("export job starting",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("method", "engine.execute"),
	)

	// Fetch the user's finance data once upfront and hand the resolved response
	// to the provider factory. The finance-backed providers (tags, budget
	// periods, default settings, and the expenses tag map) are pure mappers over
	// this response, so the export hits finance exactly once by construction.
	financeData, err := e.financeClient.GetAllUserData(ctx, &financepb.GetAllUserDataRequest{UserId: userID})
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return 0, e.recordFailure(jobID, userID, "Export timed out", err, "finance_fetch", jobStart)
		}
		return 0, e.recordFailure(jobID, userID, "Failed to fetch export data", err, "finance_fetch", jobStart)
	}
	providerSet := e.newProviders(financeData)

	// Collect from every provider concurrently. The providers are independent
	// and read-only, so each goroutine writes only its own pre-assigned index in
	// csvFiles; this makes collection latency max(providers) instead of sum while
	// keeping ZIP order deterministic (the factory's slice order).
	csvFiles := make([]CSVFile, len(providerSet))
	g, gctx := errgroup.WithContext(ctx)
	for i, provider := range providerSet {
		i, provider := i, provider
		g.Go(func() (err error) {
			// recover() does not cross goroutines and errgroup deliberately does
			// not recover (errgroup.go's own comment says propagating panics to
			// Wait "creates more problems than it solves"), so without this a
			// provider panic bypasses the job runner's recovery entirely: it kills
			// the process and leaves the job stuck in running. Returning a
			// *collectError puts it on the same path a provider error already
			// takes, so the post-Wait switch names the provider with no new
			// mapping.
			defer func() {
				if recovered := recover(); recovered != nil {
					serverkit.LogRecoveredPanic(gctx, e.logger, "goroutine.datarights_provider",
						"recovered panic in provider collection", recovered,
						slog.String("job_id", jobID),
						slog.String("user_id", userID),
						slog.String("provider", provider.Name()),
					)
					err = &collectError{
						provider: provider.Name(),
						err:      errors.New("collection failed unexpectedly"),
					}
				}
			}()

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
				// Attach the provider name before errgroup captures the error:
				// Wait surfaces only the first error, and the post-Wait switch
				// recovers the name via errors.As on *collectError.
				return &collectError{provider: provider.Name(), err: err}
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
	var ce *collectError
	switch err := g.Wait(); {
	case err == nil:
		// all providers succeeded; fall through to ZIP assembly
	case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
		return 0, e.recordFailure(jobID, userID, "Export timed out", err, "collection", jobStart)
	case errors.As(err, &ce):
		return 0, e.recordFailure(jobID, userID, fmt.Sprintf("Failed to collect %s data", ce.provider), err, "collection", jobStart)
	default:
		return 0, e.recordFailure(jobID, userID, "Failed to collect export data", err, "collection", jobStart)
	}

	// Build ZIP
	zipBytes, err := BuildZIP(csvFiles)
	if err != nil {
		return 0, e.recordFailure(jobID, userID, "Failed to build export archive", err, "zip_assembly", jobStart)
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
		return 0, e.recordFailure(jobID, userID, fmt.Sprintf("Email delivery failed: %s", sanitizeError(err)), err, "email_delivery", jobStart)
	}
	exportmetrics.ExportEmailSendDurationSeconds.Observe(time.Since(emailStart).Seconds())

	e.logger.Info("email sent",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.Float64("duration_seconds", time.Since(emailStart).Seconds()),
		slog.String("method", "engine.execute"),
	)

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

	return fileSizeBytes, nil
}

// recordFailure observes the failure metrics, logs the PII-free reason with its
// stage, and returns the terminal error the pool persists via FailJob. cause is
// the real underlying error: it gets its own server-side record and never
// reaches errMsg, because jobrunner.Pool shows errMsg to the user.
func (e *Engine) recordFailure(jobID, userID, errMsg string, cause error, stage string, jobStart time.Time) error {
	exportmetrics.ExportJobsCompletedTotal.WithLabelValues("failed").Inc()
	exportmetrics.ExportJobDurationSeconds.Observe(time.Since(jobStart).Seconds())

	e.logger.Error("export job failed",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("error", errMsg),
		slog.String("stage", stage),
		slog.String("method", "engine.execute"),
	)

	if cause != nil {
		e.logger.Error("export job failure cause",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("error", cause.Error()),
			slog.String("stage", stage),
			slog.String("method", "engine.execute"),
		)
	}

	return &failure{reason: errMsg}
}

// failure is a terminal export error whose message is the final, PII-free text
// persisted to the job's error column. Storing the text in a field (not an
// errors.New/fmt.Errorf literal) keeps the user-facing capitalized phrasing.
type failure struct{ reason string }

func (f *failure) Error() string { return f.reason }

// collectError carries the failing provider's name alongside the underlying
// cause, so the post-Wait switch recovers the name via errors.As instead of
// round-tripping structured data through the error string. It unwraps to the
// cause so errors.Is(err, context.DeadlineExceeded) still classifies timeouts.
type collectError struct {
	provider string
	err      error
}

func (e *collectError) Error() string { return "collect " + e.provider + ": " + e.err.Error() }

func (e *collectError) Unwrap() error { return e.err }

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
