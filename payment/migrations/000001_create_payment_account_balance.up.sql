CREATE TABLE IF NOT EXISTS payment_account_balance (
    account_id TEXT PRIMARY KEY,
    balance BIGINT NOT NULL CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL
);
