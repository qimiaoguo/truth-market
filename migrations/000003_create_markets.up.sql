CREATE TABLE IF NOT EXISTS markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(500) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    market_type VARCHAR(10) NOT NULL CHECK (market_type IN ('binary', 'multi')),
    category VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'closed', 'resolved', 'cancelled')),
    created_by UUID NOT NULL REFERENCES users(id),
    end_time TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_markets_status ON markets(status);
CREATE INDEX idx_markets_category ON markets(category);
CREATE INDEX idx_markets_created_at ON markets(created_at DESC);

CREATE TABLE IF NOT EXISTS outcomes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id UUID NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    label VARCHAR(200) NOT NULL,
    index INT NOT NULL,
    is_winner BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(market_id, index)
);

CREATE INDEX idx_outcomes_market_id ON outcomes(market_id);
