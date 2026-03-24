CREATE TABLE IF NOT EXISTS payment_transaction (
    id TEXT PRIMARY KEY,
    saga_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES payment_account_balance(account_id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    kind TEXT NOT NULL CHECK (kind IN ('charge', 'refund')),
    status TEXT NOT NULL CHECK (status IN ('charged', 'refunded')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (saga_id, order_id, kind)
);

CREATE INDEX IF NOT EXISTS idx_payment_transaction_order_id
    ON payment_transaction(order_id);
