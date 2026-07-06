-- Transactional, idempotent, admin-scoped cleanup of admin-owned finance data.
-- Safe to re-run: a second run deletes zero rows.
-- Uniform style: every statement uses the same inline admin subquery.
BEGIN;

DELETE FROM finance.pro_rata_schedules WHERE user_id IN (SELECT id FROM auth.users WHERE role = 'admin');
DELETE FROM finance.tags               WHERE user_id IN (SELECT id FROM auth.users WHERE role = 'admin');
DELETE FROM finance.budget_periods     WHERE user_id IN (SELECT id FROM auth.users WHERE role = 'admin');
DELETE FROM finance.default_settings   WHERE user_id IN (SELECT id FROM auth.users WHERE role = 'admin');
DELETE FROM datarights.export_jobs     WHERE user_id IN (SELECT id FROM auth.users WHERE role = 'admin');

COMMIT;
