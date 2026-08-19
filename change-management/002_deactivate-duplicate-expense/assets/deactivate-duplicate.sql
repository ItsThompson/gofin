-- Deactivates the retry duplicate (the later created_at of the pair) so the
-- active-only materialized view shows one entry. Scoped by primary key plus
-- user id: no other row can match, and a second run updates zero rows.
-- Reversible: flip status back to 'active' to restore.
-- Substitute <DUP_ID> and <USER_ID> with the values recorded in preflight.
UPDATE expenses
SET status = 'corrected'
WHERE id = '<DUP_ID>'
  AND user_id = '<USER_ID>';
