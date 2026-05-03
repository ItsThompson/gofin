-- name: GetDefaults :one
SELECT * FROM finance.default_settings WHERE user_id = $1;

-- name: UpsertDefaults :one
INSERT INTO finance.default_settings
    (user_id, budget_amount, essentials_percent, desires_percent, savings_percent, currency)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id)
DO UPDATE SET
    budget_amount = EXCLUDED.budget_amount,
    essentials_percent = EXCLUDED.essentials_percent,
    desires_percent = EXCLUDED.desires_percent,
    savings_percent = EXCLUDED.savings_percent,
    currency = EXCLUDED.currency,
    updated_at = now()
RETURNING *;

-- name: CreateTag :one
INSERT INTO finance.tags (user_id, name, is_default)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTagByID :one
SELECT * FROM finance.tags
WHERE id = $1 AND user_id = $2;

-- name: UpdateTag :one
UPDATE finance.tags SET name = $1, updated_at = now()
WHERE id = $2 AND user_id = $3
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM finance.tags WHERE id = $1 AND user_id = $2 AND is_default = false;

-- name: CountTagInProRata :one
SELECT count(*) FROM finance.pro_rata_schedules
WHERE tag_id = $1 AND user_id = $2 AND status = 'pending';

-- name: ListTags :many
SELECT * FROM finance.tags
WHERE user_id = $1
ORDER BY name ASC;

-- name: CountUserTags :one
SELECT count(*) FROM finance.tags WHERE user_id = $1;

-- name: GetCurrentPeriod :one
SELECT * FROM finance.budget_periods
WHERE user_id = $1 AND year = $2 AND month = $3;

-- name: ListPeriods :many
SELECT * FROM finance.budget_periods
WHERE user_id = $1
ORDER BY year DESC, month DESC;

-- name: CreatePeriod :one
INSERT INTO finance.budget_periods
    (user_id, year, month, budget_amount, essentials_percent, desires_percent, savings_percent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
