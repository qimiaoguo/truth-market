-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, user_id, key_hash, key_prefix, is_active, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetAPIKeyByHash :one
SELECT id, user_id, key_hash, key_prefix, is_active, expires_at, created_at
FROM api_keys
WHERE key_hash = $1;

-- name: ListAPIKeysByUser :many
SELECT id, user_id, key_hash, key_prefix, is_active, expires_at, created_at
FROM api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: RevokeAPIKey :execrows
UPDATE api_keys SET is_active = false WHERE id = $1;
