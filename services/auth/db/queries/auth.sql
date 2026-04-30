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
