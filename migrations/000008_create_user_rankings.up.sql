CREATE MATERIALIZED VIEW IF NOT EXISTS user_rankings AS
SELECT
    u.id AS user_id,
    u.wallet_address,
    u.user_type,
    u.balance + COALESCE(pos_value.total_position_value, 0) AS total_assets,
    u.balance + COALESCE(pos_value.total_position_value, 0) - 1000 AS pnl,
    COALESCE(vol.total_volume, 0) AS volume,
    CASE
        WHEN COALESCE(resolved.total_resolved, 0) = 0 THEN 0
        ELSE COALESCE(resolved.total_wins, 0)::DECIMAL / resolved.total_resolved
    END AS win_rate,
    NOW() AS updated_at
FROM users u
LEFT JOIN LATERAL (
    SELECT SUM(p.quantity * 0.5) AS total_position_value
    FROM positions p
    WHERE p.user_id = u.id AND p.quantity > 0
) pos_value ON TRUE
LEFT JOIN LATERAL (
    SELECT SUM(t.price * t.quantity) AS total_volume
    FROM trades t
    WHERE t.maker_user_id = u.id OR t.taker_user_id = u.id
) vol ON TRUE
LEFT JOIN LATERAL (
    SELECT
        COUNT(*) AS total_resolved,
        COUNT(*) FILTER (WHERE o.is_winner = TRUE AND p.quantity > 0) AS total_wins
    FROM positions p
    JOIN outcomes o ON o.id = p.outcome_id
    JOIN markets m ON m.id = p.market_id
    WHERE p.user_id = u.id AND m.status = 'resolved'
) resolved ON TRUE;

CREATE UNIQUE INDEX idx_user_rankings_user_id ON user_rankings(user_id);
CREATE INDEX idx_user_rankings_total_assets ON user_rankings(total_assets DESC);
CREATE INDEX idx_user_rankings_pnl ON user_rankings(pnl DESC);
CREATE INDEX idx_user_rankings_volume ON user_rankings(volume DESC);
CREATE INDEX idx_user_rankings_win_rate ON user_rankings(win_rate DESC);
CREATE INDEX idx_user_rankings_user_type ON user_rankings(user_type);
