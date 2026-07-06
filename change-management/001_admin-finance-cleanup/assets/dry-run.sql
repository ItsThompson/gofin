-- Read-only. Reports admin-owned rows in each cleanup target.
WITH admins AS (SELECT id FROM auth.users WHERE role = 'admin')
SELECT 'finance.pro_rata_schedules' AS table_name, count(*) FROM finance.pro_rata_schedules WHERE user_id IN (SELECT id FROM admins)
UNION ALL SELECT 'finance.tags',            count(*) FROM finance.tags            WHERE user_id IN (SELECT id FROM admins)
UNION ALL SELECT 'finance.budget_periods',  count(*) FROM finance.budget_periods  WHERE user_id IN (SELECT id FROM admins)
UNION ALL SELECT 'finance.default_settings',count(*) FROM finance.default_settings WHERE user_id IN (SELECT id FROM admins)
UNION ALL SELECT 'datarights.export_jobs',  count(*) FROM datarights.export_jobs  WHERE user_id IN (SELECT id FROM admins);
