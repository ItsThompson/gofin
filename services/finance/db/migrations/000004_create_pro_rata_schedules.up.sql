CREATE TABLE finance.pro_rata_schedules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID         NOT NULL,
    pro_rata_group    UUID         NOT NULL,
    name              VARCHAR(255) NOT NULL,
    amount            BIGINT       NOT NULL,
    currency          VARCHAR(3)   NOT NULL,
    expense_type      VARCHAR(20)  NOT NULL
                      CHECK (expense_type IN ('essentials', 'desires', 'savings')),
    tag_id            UUID         NOT NULL,
    target_year       INTEGER      NOT NULL,
    target_month      INTEGER      NOT NULL CHECK (target_month BETWEEN 1 AND 12),
    installment_index INTEGER      NOT NULL,
    installment_total INTEGER      NOT NULL,
    status            VARCHAR(10)  NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'applied')),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    applied_at        TIMESTAMPTZ,

    UNIQUE (pro_rata_group, installment_index)
);

CREATE INDEX idx_prorata_pending ON finance.pro_rata_schedules
    (user_id, target_year, target_month, status);
CREATE INDEX idx_prorata_group ON finance.pro_rata_schedules (pro_rata_group);
