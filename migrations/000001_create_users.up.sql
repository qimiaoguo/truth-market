CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(42) UNIQUE,
    user_type VARCHAR(10) NOT NULL DEFAULT 'human' CHECK (user_type IN ('human', 'agent')),
    balance DECIMAL(20, 8) NOT NULL DEFAULT 1000.00000000,
    locked_balance DECIMAL(20, 8) NOT NULL DEFAULT 0.00000000,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_user_type ON users(user_type);
