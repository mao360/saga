CREATE TABLE IF NOT EXISTS processed_commands (
    command_id TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL
);
