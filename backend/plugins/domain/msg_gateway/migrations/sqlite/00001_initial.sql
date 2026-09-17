-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_message_channels (
    id BIGINT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL,
    owner_scope VARCHAR(16) NOT NULL DEFAULT 'system',
    owner_id BIGINT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    credentials TEXT NOT NULL DEFAULT '',
    extra TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_channels_type ON w_message_channels (type);

CREATE TABLE IF NOT EXISTS w_message_bindings (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    platform_user_id VARCHAR(128) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_w_message_bindings_channel_platform
    ON w_message_bindings (channel_id, platform_user_id);
CREATE INDEX IF NOT EXISTS idx_w_message_bindings_user ON w_message_bindings (user_id);

CREATE TABLE IF NOT EXISTS w_message_pairing_codes (
    code VARCHAR(16) PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    platform_user_id VARCHAR(128) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_pairing_lookup
    ON w_message_pairing_codes (channel_id, platform_user_id);

CREATE TABLE IF NOT EXISTS w_push_events (
    id BIGINT PRIMARY KEY,
    event_key VARCHAR(80) NOT NULL,
    name VARCHAR(100) NOT NULL,
    task_type VARCHAR(100) NOT NULL DEFAULT '',
    channels TEXT NOT NULL DEFAULT '',
    targets TEXT NOT NULL DEFAULT '',
    template TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_w_push_events_key ON w_push_events(event_key);
CREATE INDEX IF NOT EXISTS idx_w_push_events_enabled ON w_push_events(enabled);
CREATE INDEX IF NOT EXISTS idx_w_push_events_task_type ON w_push_events(task_type);

CREATE TABLE IF NOT EXISTS w_push_channels (
    id BIGINT PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL DEFAULT 'custom',
    token VARCHAR(100) NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    other TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_w_push_channels_name ON w_push_channels(name);
CREATE INDEX IF NOT EXISTS idx_w_push_channels_enabled ON w_push_channels(enabled);

CREATE TABLE IF NOT EXISTS w_push_histories (
    id BIGINT PRIMARY KEY,
    event_key VARCHAR(80) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    target VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    level VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    error_msg TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_push_histories_event ON w_push_histories(event_key);
CREATE INDEX IF NOT EXISTS idx_w_push_histories_created ON w_push_histories(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_push_histories;
DROP TABLE IF EXISTS w_push_channels;
DROP TABLE IF EXISTS w_push_events;
DROP TABLE IF EXISTS w_message_pairing_codes;
DROP TABLE IF EXISTS w_message_bindings;
DROP TABLE IF EXISTS w_message_channels;
-- +goose StatementEnd
