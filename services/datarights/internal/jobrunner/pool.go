// Package jobrunner provides the bounded-worker-pool lifecycle shared by the
// datarights export and deletion engines. Both engines run structurally
// identical pools (a semaphore, a running-status transition, a timeout-derived
// context, and a background-context fail-on-timeout) and differ only in the
// per-job work they perform. That work is injected as an [Execute] strategy;
// the pool owns everything around it.
//
// jobrunner is internal to datarights: it is imported only by
// internal/deletion and internal/engine and is not promoted to a workspace
// module.
package jobrunner

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// runningStatus is the status a job is transitioned to before its work runs.
const runningStatus = "running"

// StatusStore persists a job's lifecycle transitions. The deletion job repo
// satisfies it directly; the export engine adapts its repo behind this seam so
// its CompleteJob can still persist file_size_bytes.
type StatusStore interface {
	UpdateStatus(ctx context.Context, jobID, status string) error
	FailJob(ctx context.Context, jobID, reason string) error
	CompleteJob(ctx context.Context, jobID string) error
}

// Execute is the injected per-job strategy. Export injects its errgroup fan-out
// + zip + email closure; deletion injects its sequential-retry-with-backoff
// closure. The pool owns slot acquisition, the running transition, and terminal
// persistence around this call; the strategy owns the work and returns the
// PII-free failure reason (its Error string is persisted verbatim).
type Execute func(ctx context.Context, jobID, userID string) error

// Pool owns the bounded-worker-pool lifecycle. A single Pool is created per
// engine and reused across all of that engine's jobs.
type Pool struct {
	sem     chan struct{}
	queued  atomic.Int64
	timeout time.Duration
	store   StatusStore
	execute Execute
	log     *slog.Logger
}

// New creates a Pool bounded to maxConcurrent in-flight jobs, each run with the
// given per-job timeout, persisting transitions through store and performing
// work via execute.
func New(maxConcurrent int, timeout time.Duration, store StatusStore, execute Execute, log *slog.Logger) *Pool {
	return &Pool{
		sem:     make(chan struct{}, maxConcurrent),
		timeout: timeout,
		store:   store,
		execute: execute,
		log:     log,
	}
}

// Submit is non-blocking: it spawns a worker goroutine that acquires a slot,
// transitions the job to running, runs execute, then completes or fails it.
func (p *Pool) Submit(jobID, userID string) {
	go p.run(jobID, userID)
}

func (p *Pool) run(jobID, userID string) {
	p.queued.Add(1)
	p.sem <- struct{}{} // acquire slot (blocks if the pool is full)
	p.queued.Add(-1)
	defer func() { <-p.sem }()

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	if err := p.store.UpdateStatus(ctx, jobID, runningStatus); err != nil {
		p.log.Error("failed to mark job running",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return
	}

	if err := p.execute(ctx, jobID, userID); err != nil {
		// The job context may already be expired (timeout), so persist the
		// terminal failure through a fresh background context.
		if failErr := p.store.FailJob(context.Background(), jobID, err.Error()); failErr != nil {
			p.log.Error("failed to mark job failed",
				slog.String("job_id", jobID),
				slog.String("user_id", userID),
				slog.String("error", failErr.Error()),
			)
		}
		return
	}

	if err := p.store.CompleteJob(ctx, jobID); err != nil {
		p.log.Error("failed to complete job",
			slog.String("job_id", jobID),
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
	}
}

// ActiveJobs returns the number of jobs currently holding a pool slot.
func (p *Pool) ActiveJobs() int { return len(p.sem) }

// QueuedJobs returns the number of submitted jobs still waiting for a slot.
func (p *Pool) QueuedJobs() int { return int(p.queued.Load()) }

// MaxConcurrent returns the pool capacity.
func (p *Pool) MaxConcurrent() int { return cap(p.sem) }
