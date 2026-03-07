-- name: CreateOrder :exec
INSERT INTO orders (id, user_id, market_id, outcome_id, side, price, quantity,
    filled_quantity, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetOrderByID :one
SELECT id, user_id, market_id, outcome_id, side, price, quantity,
       filled_quantity, status, created_at, updated_at
FROM orders
WHERE id = $1;

-- name: UpdateOrderStatus :execrows
UPDATE orders SET status = $1, filled_quantity = $2, updated_at = NOW() WHERE id = $3;

-- name: ListOpenOrdersByMarket :many
SELECT id, user_id, market_id, outcome_id, side, price, quantity,
       filled_quantity, status, created_at, updated_at
FROM orders
WHERE market_id = $1 AND status IN ('open', 'partially_filled')
ORDER BY price DESC, created_at ASC;

-- name: ListAllOpenOrders :many
SELECT id, user_id, market_id, outcome_id, side, price, quantity,
       filled_quantity, status, created_at, updated_at
FROM orders
WHERE status IN ('open', 'partially_filled')
ORDER BY market_id, outcome_id, created_at ASC;

-- name: CountOrdersByUser :one
SELECT COUNT(*) FROM orders WHERE user_id = $1;

-- name: ListOrdersByUser :many
SELECT id, user_id, market_id, outcome_id, side, price, quantity,
       filled_quantity, status, created_at, updated_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CancelAllOrdersByMarket :execrows
UPDATE orders SET status = 'cancelled', updated_at = NOW()
WHERE market_id = $1 AND status IN ('open', 'partially_filled');
