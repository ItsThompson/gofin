-- Down migration for reporting_currency.
--
-- WARNING: this down migration removes the reporting_currency column and the
-- migration report table. It does NOT preserve period-currency history: once
-- the column is dropped, the association between each budget period and its
-- reporting currency is lost. Re-running the up migration would re-derive
-- reporting currencies from the current default/auth/fallback sources, which
-- may differ from the values present before the downgrade if defaults changed
-- in the meantime. Proceed only if you accept this data loss.

ALTER TABLE finance.budget_periods
DROP CONSTRAINT IF EXISTS budget_periods_reporting_currency_supported;

ALTER TABLE finance.budget_periods
DROP COLUMN reporting_currency;

DROP TABLE IF EXISTS finance.period_reporting_currency_migration_report;