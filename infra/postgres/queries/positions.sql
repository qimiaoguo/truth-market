-- name: UpsertPosition :exec
INSERT INTO positions (id, user_id, market_id, outcome_id, quantity, avg_price, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, market_id, outcome_id) DO UPDATE SET
    quantity = EXCLUDED.quantity,
    avg_price = EXCLUDED.avg_price,
    updated_at = EXCLUDED.updated_at;

-- name: GetPositionByUserAndOutcome :one
SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
FROM positions
WHERE user_id = $1 AND outcome_id = $2;

-- name: ListPositionsByUser :many
SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
FROM positions
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: ListPositionsByMarket :many
SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
FROM positions
WHERE market_id = $1
ORDER BY updated_at DESC;
