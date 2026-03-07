-- name: CreateOutcomeBatch :copyfrom
INSERT INTO outcomes (id, market_id, label, index, is_winner) VALUES ($1, $2, $3, $4, $5);

-- name: ListOutcomesByMarket :many
SELECT id, market_id, label, index, is_winner
FROM outcomes
WHERE market_id = $1
ORDER BY index ASC;

-- name: SetOutcomeWinner :execrows
UPDATE outcomes SET is_winner = true WHERE id = $1;
