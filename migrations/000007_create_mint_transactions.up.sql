CREATE TABLE IF NOT EXISTS mint_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    market_id UUID NOT NULL REFERENCES markets(id),
    quantity DECIMAL(20, 8) NOT NULL,
    cost DECIMAL(20, 8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mint_transactions_user_id ON mint_transactions(user_id);
CREATE INDEX idx_mint_transactions_market_id ON mint_transactions(market_id);
