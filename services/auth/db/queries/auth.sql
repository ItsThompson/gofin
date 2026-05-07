-- name: CreateUser :one
INSERT INTO auth.users (username, email, password_hash, role, currency)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM auth.users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM auth.users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM auth.users WHERE username = $1;

-- name: BlacklistToken :exec
INSERT INTO auth.refresh_token_blacklist (jti, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: IsTokenBlacklisted :one
SELECT EXISTS(
    SELECT 1 FROM auth.refresh_token_blacklist WHERE jti = $1
) AS is_blacklisted;

-- name: CleanupExpiredBlacklist :exec
DELETE FROM auth.refresh_token_blacklist WHERE expires_at < now();

-- name: CompleteOnboarding :one
UPDATE auth.users
SET has_completed_onboarding = true, currency = $1, updated_at = now()
WHERE id = $2
RETURNING *;

-- name: ListAllUsers :many
SELECT id, username, email, role, created_at
FROM auth.users
ORDER BY created_at ASC;

-- name: UpdateUser :one
UPDATE auth.users
SET username = $1, email = $2, currency = $3, updated_at = now()
WHERE id = $4
RETURNING *;

-- name: UpdatePassword :exec
UPDATE auth.users
SET password_hash = $1, updated_at = now()
WHERE id = $2;

-- name: RevokeAllUserTokens :exec
UPDATE auth.users
SET tokens_revoked_at = now(), updated_at = now()
WHERE id = $1;

-- name: GetTokensRevokedAt :one
SELECT tokens_revoked_at FROM auth.users WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM auth.users WHERE id = $1;
