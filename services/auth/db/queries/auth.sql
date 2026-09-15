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
VALUES ($1, $2, $3)
ON CONFLICT (jti) DO NOTHING;

-- name: ConsumeRefreshToken :one
INSERT INTO auth.refresh_token_blacklist (jti, user_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (jti) DO NOTHING
RETURNING jti;

-- name: CleanupExpiredBlacklist :exec
DELETE FROM auth.refresh_token_blacklist WHERE expires_at < now();

-- name: CompleteOnboarding :one
UPDATE auth.users
SET has_completed_onboarding = true,
    onboarding_completed_at = COALESCE(onboarding_completed_at, now()),
    currency = $1,
    updated_at = now()
WHERE id = $2
RETURNING *;

-- name: CountUsersCreatedInWindows :one
SELECT
    count(*) FILTER (WHERE created_at >= $1 AND created_at < $2) AS report_week,
    count(*) FILTER (WHERE created_at >= $3 AND created_at < $4) AS previous_week,
    count(*) FILTER (WHERE created_at >= $5 AND created_at < $6) AS trailing_four_weeks_total
FROM auth.users
WHERE created_at >= $5 AND created_at < $2;

-- name: CountOnboardingCompletionsInWindows :one
SELECT
    count(*) FILTER (WHERE onboarding_completed_at >= $1 AND onboarding_completed_at < $2) AS report_week,
    count(*) FILTER (WHERE onboarding_completed_at >= $3 AND onboarding_completed_at < $4) AS previous_week,
    count(*) FILTER (WHERE onboarding_completed_at >= $5 AND onboarding_completed_at < $6) AS trailing_four_weeks_total
FROM auth.users
WHERE onboarding_completed_at >= $5 AND onboarding_completed_at < $2;

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

-- name: DeleteRefreshTokenBlacklist :exec
DELETE FROM auth.refresh_token_blacklist WHERE user_id = $1;

-- name: DeleteUser :exec
DELETE FROM auth.users WHERE id = $1;
