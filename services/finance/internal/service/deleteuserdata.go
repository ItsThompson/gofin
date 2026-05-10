package service

import (
	"context"
	"fmt"
)

// DeleteAllUserData removes all finance data for a user within a single transaction.
// Deletes from: pro_rata_schedules, tags, budget_periods, default_settings.
// Idempotent: returns nil when the user has no finance data (0 rows deleted).
// Used by the datarights service for GDPR user deletion.
func (s *FinanceService) DeleteAllUserData(ctx context.Context, userID string) error {
	tx, err := s.txBeginner.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := tx.Repo()
	if err := txRepo.DeleteAllUserData(ctx, userID); err != nil {
		return fmt.Errorf("deleting all user data: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	s.logger.Info("all user data deleted",
		"method", "DeleteAllUserData",
		"user_id", userID,
	)

	return nil
}
