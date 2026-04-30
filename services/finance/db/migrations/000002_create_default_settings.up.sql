CREATE TABLE finance.default_settings (
    user_id             UUID PRIMARY KEY,
    budget_amount       BIGINT       NOT NULL DEFAULT 0,
    essentials_percent  INTEGER      NOT NULL DEFAULT 50,
    desires_percent     INTEGER      NOT NULL DEFAULT 30,
    savings_percent     INTEGER      NOT NULL DEFAULT 20,
    currency            VARCHAR(3)   NOT NULL DEFAULT 'USD',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CHECK (essentials_percent + desires_percent + savings_percent = 100)
);
