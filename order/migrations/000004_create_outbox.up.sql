CREATE TABLE IF NOT EXISTS outbox_messages (
    id         UUID        PRIMARY KEY,
    topic      TEXT        NOT NULL,
    key        TEXT        NOT NULL,
    payload    JSONB       NOT NULL,
    headers    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at    TIMESTAMPTZ,
    attempts   INT         NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_messages_unsent
    ON outbox_messages (created_at)
    WHERE sent_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_messages_sent_at
    ON outbox_messages (sent_at)
    WHERE sent_at IS NOT NULL;