ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS sku        TEXT   NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS qty        BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS account_id TEXT   NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status     TEXT   NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'failed'));