package repository

import (
	"context"
	"fmt"
	"log/slog"
)

// InitSchema creates the expenses table and indexes if they don't exist, and
// reconciles additive money-snapshot columns onto pre-existing tables.
func (r *ImmudbExpenseRepository) InitSchema(ctx context.Context) error {
	createTable := `CREATE TABLE IF NOT EXISTS expenses (
		id              VARCHAR(36)  NOT NULL,
		user_id         VARCHAR(36)  NOT NULL,
		name            VARCHAR(255) NOT NULL,
		expense_type    VARCHAR(20)  NOT NULL,
		tag_id          VARCHAR(36)  NOT NULL,
		expense_date    VARCHAR(10)  NOT NULL,
		period_year     INTEGER      NOT NULL,
		period_month    INTEGER      NOT NULL,
		status          VARCHAR(20)  NOT NULL,
		corrects_id     VARCHAR(36),
		is_pro_rata     BOOLEAN      NOT NULL,
		pro_rata_group  VARCHAR(36),
		pro_rata_index  INTEGER,
		pro_rata_total  INTEGER,
		created_at      VARCHAR(30)  NOT NULL,
		transaction_amount      INTEGER,
		transaction_currency    VARCHAR(3),
		reporting_amount        INTEGER,
		reporting_currency      VARCHAR(3),
		exchange_rate           VARCHAR(40),
		exchange_rate_source    VARCHAR(40),
		exchange_rate_timestamp VARCHAR(30),
		exchange_rate_expires_at VARCHAR(30),
		idempotency_key         VARCHAR(36),
		PRIMARY KEY (id)
	);`

	_, err := r.client.SQLExec(ctx, createTable, nil)
	if err != nil {
		return fmt.Errorf("creating expenses table: %w", err)
	}

	// Reconcile additive money-snapshot columns onto tables created before the
	// multi-currency cutover. CREATE TABLE IF NOT EXISTS is a no-op for an existing
	// table, so the new nullable columns would be missing on pre-cutover tables.
	// ALTER TABLE ADD COLUMN adds them; a column that already exists returns an
	// error, which we swallow so reconciliation is idempotent.
	reconcileColumns := []string{
		`ALTER TABLE expenses ADD COLUMN transaction_amount INTEGER;`,
		`ALTER TABLE expenses ADD COLUMN transaction_currency VARCHAR(3);`,
		`ALTER TABLE expenses ADD COLUMN reporting_amount INTEGER;`,
		`ALTER TABLE expenses ADD COLUMN reporting_currency VARCHAR(3);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate VARCHAR(40);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_source VARCHAR(40);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_timestamp VARCHAR(30);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_expires_at VARCHAR(30);`,
		`ALTER TABLE expenses ADD COLUMN idempotency_key VARCHAR(36);`,
	}
	for _, stmt := range reconcileColumns {
		if _, addErr := r.client.SQLExec(ctx, stmt, nil); addErr != nil {
			// Expected when the column already exists (idempotent reconcile).
			r.logger.Debug("snapshot column reconcile skipped (may already exist)",
				slog.String("statement", stmt),
				slog.String("error", addErr.Error()),
			)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_period ON expenses (user_id, period_year, period_month, status);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_corrects ON expenses (corrects_id);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_prorata_group ON expenses (pro_rata_group);`,
		// Covers the keyset export seek: the (created_at, id) tiebreaker column is
		// included so the index matches the full ORDER BY created_at ASC, id ASC
		// tuple and rows tied on created_at seek instead of scanning+sorting.
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_created_id ON expenses (user_id, created_at, id);`,
		// A regular (non-unique) index because immudb 1.11.0 only supports
		// unique index creation on empty tables; the prod table has data.
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_idem ON expenses (user_id, idempotency_key);`,
	}

	for _, idx := range indexes {
		_, err := r.client.SQLExec(ctx, idx, nil)
		if err != nil {
			// Index creation may fail if it already exists; log and continue.
			r.logger.Warn("index creation skipped (may already exist)",
				slog.String("error", err.Error()),
			)
		}
	}

	r.logger.Info("immudb schema initialized")
	return nil
}
