-- +goose Up
CREATE TABLE w_push_events (
    id BIGSERIAL PRIMARY KEY,
    event_key VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    channels TEXT NOT NULL,
    targets TEXT NOT NULL,
    template TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_w_push_events_enabled ON w_push_events(enabled);

CREATE TABLE w_push_histories (
    id BIGSERIAL PRIMARY KEY,
    event_key VARCHAR(80) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    target VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    level VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    error_msg TEXT,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_w_push_histories_event ON w_push_histories(event_key);
CREATE INDEX idx_w_push_histories_created ON w_push_histories(created_at);

-- +goose Down
DROP TABLE IF EXISTS w_push_histories;
DROP TABLE IF EXISTS w_push_events;
