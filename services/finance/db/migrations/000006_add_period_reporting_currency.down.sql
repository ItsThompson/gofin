ALTER TABLE finance.budget_periods
DROP CONSTRAINT IF EXISTS budget_periods_reporting_currency_supported;

ALTER TABLE finance.budget_periods
DROP COLUMN reporting_currency;

DROP TABLE IF EXISTS finance.period_reporting_currency_migration_report;
