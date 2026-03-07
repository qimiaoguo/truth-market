-- name: CreateMarket :exec
INSERT INTO markets (id, title, description, category, market_type, status,
    created_by, resolved_outcome_id, created_at, updated_at, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetMarketByID :one
SELECT id, title, description, category, market_type, status,
       created_by, resolved_outcome_id, created_at, updated_at, end_time
FROM markets
WHERE id = $1;

-- name: UpdateMarket :execrows
UPDATE markets SET
    title = $1, description = $2, category = $3, market_type = $4, status = $5,
    created_by = $6, resolved_outcome_id = $7, updated_at = $8, end_time = $9
WHERE id = $10;

-- name: CountMarkets :one
SELECT COUNT(*) FROM markets
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'));

-- name: ListMarkets :many
SELECT id, title, description, category, market_type, status,
       created_by, resolved_outcome_id, created_at, updated_at, end_time
FROM markets
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
