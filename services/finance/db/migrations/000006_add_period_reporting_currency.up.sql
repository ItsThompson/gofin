ALTER TABLE finance.budget_periods
ADD COLUMN reporting_currency VARCHAR(3);

CREATE TEMP TABLE period_reporting_currency_supported (
    code VARCHAR(3) PRIMARY KEY
);

INSERT INTO period_reporting_currency_supported (code)
VALUES
    ('USD'),
    ('EUR'),
    ('GBP'),
    ('JPY'),
    ('CAD'),
    ('AUD'),
    ('CHF'),
    ('CNY'),
    ('SGD'),
    ('HKD');

CREATE TEMP TABLE period_reporting_currency_backfill (
    period_id          UUID PRIMARY KEY,
    reporting_currency VARCHAR(3) NOT NULL,
    source             TEXT       NOT NULL
);

INSERT INTO period_reporting_currency_backfill (period_id, reporting_currency, source)
SELECT period.id, supported.code, 'finance_default'
FROM finance.budget_periods AS period
JOIN finance.default_settings AS defaults ON defaults.user_id = period.user_id
JOIN period_reporting_currency_supported AS supported ON supported.code = UPPER(defaults.currency);

DO $$
BEGIN
    IF to_regclass('auth.users') IS NOT NULL THEN
        EXECUTE $backfill_auth$
            INSERT INTO pg_temp.period_reporting_currency_backfill (period_id, reporting_currency, source)
            SELECT period.id, supported.code, 'auth_user'
            FROM finance.budget_periods AS period
            JOIN auth.users AS users ON users.id = period.user_id
            JOIN pg_temp.period_reporting_currency_supported AS supported ON supported.code = UPPER(users.currency)
            WHERE NOT EXISTS (
                SELECT 1
                FROM pg_temp.period_reporting_currency_backfill AS backfill
                WHERE backfill.period_id = period.id
            )
        $backfill_auth$;
    END IF;
END $$;

INSERT INTO period_reporting_currency_backfill (period_id, reporting_currency, source)
SELECT period.id, 'USD', 'fallback'
FROM finance.budget_periods AS period
WHERE NOT EXISTS (
    SELECT 1
    FROM period_reporting_currency_backfill AS backfill
    WHERE backfill.period_id = period.id
);

CREATE TABLE finance.period_reporting_currency_migration_report (
    period_id          UUID PRIMARY KEY,
    user_id            UUID        NOT NULL,
    reporting_currency VARCHAR(3)  NOT NULL,
    reason             TEXT        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO finance.period_reporting_currency_migration_report (period_id, user_id, reporting_currency, reason)
SELECT period.id, period.user_id, backfill.reporting_currency, 'no_supported_default_or_auth_currency'
FROM finance.budget_periods AS period
JOIN period_reporting_currency_backfill AS backfill ON backfill.period_id = period.id
WHERE backfill.source = 'fallback';

UPDATE finance.budget_periods AS period
SET reporting_currency = backfill.reporting_currency
FROM period_reporting_currency_backfill AS backfill
WHERE backfill.period_id = period.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM finance.budget_periods AS period
        LEFT JOIN period_reporting_currency_supported AS supported ON supported.code = period.reporting_currency
        WHERE supported.code IS NULL
    ) THEN
        RAISE EXCEPTION 'budget period reporting currency backfill produced unsupported values';
    END IF;
END $$;

ALTER TABLE finance.budget_periods
ALTER COLUMN reporting_currency SET NOT NULL;

ALTER TABLE finance.budget_periods
ADD CONSTRAINT budget_periods_reporting_currency_supported
CHECK (reporting_currency IN ('USD', 'EUR', 'GBP', 'JPY', 'CAD', 'AUD', 'CHF', 'CNY', 'SGD', 'HKD'));
