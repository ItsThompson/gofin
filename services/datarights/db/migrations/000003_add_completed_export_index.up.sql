CREATE INDEX idx_export_jobs_completed_at
    ON datarights.export_jobs (completed_at)
    WHERE status = 'completed';
