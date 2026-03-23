CREATE TABLE IF NOT EXISTS inventory_stock (
    sku TEXT PRIMARY KEY,
    total_qty BIGINT NOT NULL CHECK (total_qty >= 0),
    available_qty BIGINT NOT NULL CHECK (available_qty >= 0 AND available_qty <= total_qty),
    updated_at TIMESTAMPTZ NOT NULL
);
