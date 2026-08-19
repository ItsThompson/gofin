-- Post-check. Substitute <USER_ID> with the owner's auth.users id.
-- Expect: exactly one active row (the kept id) and one corrected row (the
-- duplicate).
SELECT id, name, amount, expense_date, status, created_at
FROM expenses
WHERE user_id = '<USER_ID>'
  AND name = 'British Airlines Wifi'
  AND expense_date = '2026-08-18'
ORDER BY created_at ASC;
