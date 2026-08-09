package service

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ItsThompson/gofin/services/serverkit"
)

// StartPeriodicCleanup launches a background goroutine that cleans up expired
// blacklist entries on a fixed interval. It respects the provided context for
// graceful shutdown and ensures only one cleanup runs at a time.
func (s *AuthService) StartPeriodicCleanup(ctx context.Context, interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		var running atomic.Bool

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("periodic cleanup stopped",
					slog.String("method", "StartPeriodicCleanup"),
				)
				return
			case <-ticker.C:
				if !running.CompareAndSwap(false, true) {
					s.logger.Debug("cleanup skipped: previous run still in progress",
						slog.String("method", "StartPeriodicCleanup"),
					)
					continue
				}

				go func() {
					defer running.Store(false)

					// The repo call runs outside any request recovery, so an
					// unrecovered panic here takes the whole auth process down.
					// Recovering per run keeps the ticker alive, so the next tick
					// retries.
					defer func() {
						if recovered := recover(); recovered != nil {
							serverkit.LogRecoveredPanic(ctx, s.logger, "goroutine.auth_blacklist_cleanup",
								"recovered panic in blacklist cleanup", recovered,
								slog.String("method", "StartPeriodicCleanup"),
							)
						}
					}()

					cleanupCtx, cancel := context.WithTimeout(ctx, timeout)
					defer cancel()

					if err := s.blacklistRepo.CleanupExpired(cleanupCtx); err != nil {
						s.logger.Error("blacklist cleanup failed",
							slog.String("method", "StartPeriodicCleanup"),
							slog.String("error", err.Error()),
						)
						return
					}

					s.logger.Debug("blacklist cleanup completed",
						slog.String("method", "StartPeriodicCleanup"),
					)
				}()
			}
		}
	}()
}
