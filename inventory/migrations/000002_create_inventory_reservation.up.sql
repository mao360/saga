CREATE TABLE IF NOT EXISTS inventory_reservation (
    id TEXT PRIMARY KEY,
    saga_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    sku TEXT NOT NULL REFERENCES inventory_stock(sku),
    qty BIGINT NOT NULL CHECK (qty > 0),
    status TEXT NOT NULL CHECK (status IN ('reserved', 'released')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (saga_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_inventory_reservation_order_id
    ON inventory_reservation(order_id);
