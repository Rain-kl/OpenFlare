-- +goose Up
CREATE TABLE w_push_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    type TEXT NOT NULL DEFAULT 'custom',
    token TEXT,
    url TEXT NOT NULL,
    other TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_w_push_channels_name ON w_push_channels(name);
CREATE INDEX idx_w_push_channels_enabled ON w_push_channels(enabled);

-- +goose Down
DROP TABLE IF EXISTS w_push_channels;
