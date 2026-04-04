CREATE TABLE IF NOT EXISTS saga_state (
    saga_id          TEXT PRIMARY KEY,
    order_id         TEXT NOT NULL REFERENCES orders(id),
    inventory_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (inventory_status IN ('pending', 'reserved', 'rejected', 'released')),
    payment_status   TEXT NOT NULL DEFAULT 'pending'
        CHECK (payment_status IN ('pending', 'charged', 'rejected', 'refunded')),
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_saga_state_order_id ON saga_state(order_id);