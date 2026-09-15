DROP INDEX IF EXISTS auth.idx_users_reporting_onboarding_completed_at;
DROP INDEX IF EXISTS auth.idx_users_reporting_created_at;

ALTER TABLE auth.users
    DROP COLUMN onboarding_completed_at;
