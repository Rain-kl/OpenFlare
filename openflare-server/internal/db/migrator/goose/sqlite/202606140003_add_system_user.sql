-- +goose Up
INSERT INTO w_users (id, username, password, nickname, avatar_url, is_active, is_admin, last_login_at, created_at, updated_at)
VALUES (999, 'system', '*', '系统', '', TRUE, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (username) DO NOTHING;

-- +goose Down
DELETE FROM w_users WHERE username = 'system';
