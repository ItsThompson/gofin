CREATE SCHEMA IF NOT EXISTS finance;

CREATE TABLE finance.budget_periods (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID         NOT NULL,
    year                INTEGER      NOT NULL,
    month               INTEGER      NOT NULL CHECK (month BETWEEN 1 AND 12),
    budget_amount       BIGINT       NOT NULL,
    essentials_percent  INTEGER      NOT NULL CHECK (essentials_percent BETWEEN 0 AND 100),
    desires_percent     INTEGER      NOT NULL CHECK (desires_percent BETWEEN 0 AND 100),
    savings_percent     INTEGER      NOT NULL CHECK (savings_percent BETWEEN 0 AND 100),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE (user_id, year, month),
    CHECK (essentials_percent + desires_percent + savings_percent = 100)
);

CREATE INDEX idx_periods_user ON finance.budget_periods (user_id, year DESC, month DESC);
