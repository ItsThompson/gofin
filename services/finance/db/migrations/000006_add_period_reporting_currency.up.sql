ALTER TABLE finance.budget_periods
ADD COLUMN reporting_currency VARCHAR(3);

UPDATE finance.budget_periods AS period
SET reporting_currency = UPPER(COALESCE(defaults.currency, 'USD'))
FROM finance.default_settings AS defaults
WHERE period.user_id = defaults.user_id;

UPDATE finance.budget_periods
SET reporting_currency = 'USD'
WHERE reporting_currency IS NULL;

ALTER TABLE finance.budget_periods
ALTER COLUMN reporting_currency SET NOT NULL;
