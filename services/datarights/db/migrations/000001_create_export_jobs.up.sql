CREATE SCHEMA IF NOT EXISTS datarights;

CREATE TABLE datarights.export_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    error           TEXT,
    file_size_bytes BIGINT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_export_jobs_user_status ON datarights.export_jobs (user_id, status);
CREATE INDEX idx_export_jobs_user_created ON datarights.export_jobs (user_id, created_at DESC);
