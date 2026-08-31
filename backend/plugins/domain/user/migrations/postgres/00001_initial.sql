-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_users (
    id BIGINT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(255),
    nickname VARCHAR(255),
    email VARCHAR(255),
    avatar_url VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    is_admin BOOLEAN DEFAULT FALSE,
    bio VARCHAR(500),
    phone VARCHAR(32),
    gender VARCHAR(16),
    website VARCHAR(255),
    location VARCHAR(255),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_users_email ON w_users (email);
CREATE INDEX IF NOT EXISTS idx_w_users_is_active ON w_users (is_active);
CREATE INDEX IF NOT EXISTS idx_w_users_last_login_at ON w_users (last_login_at);
CREATE INDEX IF NOT EXISTS idx_w_users_created_at ON w_users (created_at);

-- Seed system user
INSERT INTO w_users (id, username, password, nickname, avatar_url, is_active, is_admin, last_login_at, created_at, updated_at)
VALUES (999, 'system', '*', '系统', '', TRUE, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING;

-- Seed default administrator user (username: admin, password: 12345678)
INSERT INTO w_users (id, username, password, nickname, email, is_active, is_admin, last_login_at, created_at, updated_at)
VALUES (1, 'admin', '12345678', '管理员', 'admin@wavelet.local', TRUE, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM w_users WHERE username = 'system';
DROP TABLE IF EXISTS w_users;
-- +goose StatementEnd
