-- name: CreateTrade :exec
INSERT INTO trades (id, market_id, outcome_id, maker_order_id, taker_order_id,
    maker_user_id, taker_user_id, price, quantity, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: CountTradesByMarket :one
SELECT COUNT(*) FROM trades WHERE market_id = $1;

-- name: ListTradesByMarket :many
SELECT id, market_id, outcome_id, maker_order_id, taker_order_id,
       maker_user_id, taker_user_id, price, quantity, created_at
FROM trades
WHERE market_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountTradesByUser :one
SELECT COUNT(*) FROM trades WHERE maker_user_id = $1 OR taker_user_id = $1;

-- name: ListTradesByUser :many
SELECT id, market_id, outcome_id, maker_order_id, taker_order_id,
       maker_user_id, taker_user_id, price, quantity, created_at
FROM trades
WHERE maker_user_id = $1 OR taker_user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
