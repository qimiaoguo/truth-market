-- name: CreateMintTransaction :exec
INSERT INTO mint_transactions (id, user_id, market_id, quantity, cost, created_at)
VALUES ($1, $2, $3, $4, $5, $6);
