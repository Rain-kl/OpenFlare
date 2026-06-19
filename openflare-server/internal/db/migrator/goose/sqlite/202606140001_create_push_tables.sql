-- +goose Up
CREATE TABLE w_push_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    channels TEXT NOT NULL,
    targets TEXT NOT NULL,
    template TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_w_push_events_enabled ON w_push_events(enabled);

CREATE TABLE w_push_histories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_key TEXT NOT NULL,
    channel TEXT NOT NULL,
    target TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    level TEXT NOT NULL,
    status TEXT NOT NULL,
    error_msg TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_w_push_histories_event ON w_push_histories(event_key);
CREATE INDEX idx_w_push_histories_created ON w_push_histories(created_at);

-- +goose Down
DROP TABLE IF EXISTS w_push_histories;
DROP TABLE IF EXISTS w_push_events;
