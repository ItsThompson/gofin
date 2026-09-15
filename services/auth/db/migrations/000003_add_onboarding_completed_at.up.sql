ALTER TABLE auth.users
    ADD COLUMN onboarding_completed_at TIMESTAMPTZ;

WITH completion_time AS (
    SELECT now() AS completed_at
)
UPDATE auth.users AS users
SET onboarding_completed_at = completion_time.completed_at
FROM completion_time
WHERE users.has_completed_onboarding
  AND users.onboarding_completed_at IS NULL;

CREATE INDEX idx_users_reporting_created_at ON auth.users (created_at);
CREATE INDEX idx_users_reporting_onboarding_completed_at ON auth.users (onboarding_completed_at);
