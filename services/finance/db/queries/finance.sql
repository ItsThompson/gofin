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

-- name: ListTags :many
SELECT * FROM finance.tags
WHERE user_id = $1
ORDER BY name ASC;

-- name: CountUserTags :one
SELECT count(*) FROM finance.tags WHERE user_id = $1;
