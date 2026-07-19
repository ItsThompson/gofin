CREATE TABLE finance.health_scores (
    user_id         UUID         NOT NULL,
    year            INTEGER      NOT NULL,
    month           INTEGER      NOT NULL CHECK (month BETWEEN 1 AND 12),
    total           INTEGER      NOT NULL CHECK (total BETWEEN 0 AND 100),
    band            TEXT         NOT NULL,
    score           JSONB        NOT NULL,
    formula_version INTEGER      NOT NULL,
    computed_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE (user_id, year, month)
);

CREATE INDEX idx_health_scores_user ON finance.health_scores (user_id, year DESC, month DESC);
