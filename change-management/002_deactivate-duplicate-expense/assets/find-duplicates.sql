-- Read-only. Finds the duplicate pair for the reported double-submission.
-- Substitute <USER_ID> with the owner's auth.users id from Postgres before
-- running. Expect exactly two rows, both status 'active', differing only in
-- id and created_at; pro_rata_group must be NULL on both.
SELECT id, user_id, name, amount, currency, expense_type, tag_id,
       expense_date, period_year, period_month, status, corrects_id,
       is_pro_rata, pro_rata_group, created_at
FROM expenses
WHERE user_id = '<USER_ID>'
  AND name = 'British Airlines Wifi'
  AND expense_date = '2026-08-18'
  AND status = 'active'
ORDER BY created_at ASC;
