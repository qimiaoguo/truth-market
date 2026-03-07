-- name: CreateUser :exec
INSERT INTO users (id, wallet_address, user_type, balance, locked_balance, is_admin, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetUserByID :one
SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
FROM users
WHERE id = $1;

-- name: GetUserByWallet :one
SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
FROM users
WHERE wallet_address = $1;

-- name: UpdateUserBalance :execrows
UPDATE users SET balance = $1, locked_balance = $2 WHERE id = $3;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.narg('user_type')::text IS NULL OR user_type = sqlc.narg('user_type'))
  AND (sqlc.narg('is_admin')::boolean IS NULL OR is_admin = sqlc.narg('is_admin'));

-- name: ListUsers :many
SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
FROM users
WHERE (sqlc.narg('user_type')::text IS NULL OR user_type = sqlc.narg('user_type'))
  AND (sqlc.narg('is_admin')::boolean IS NULL OR is_admin = sqlc.narg('is_admin'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
