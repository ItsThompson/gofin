CREATE TABLE datarights.deletion_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL,
    admin_user_id   UUID         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    error           TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_deletion_jobs_user_status ON datarights.deletion_jobs (user_id, status);
CREATE INDEX idx_deletion_jobs_status ON datarights.deletion_jobs (status)
    WHERE status IN ('pending', 'running');
