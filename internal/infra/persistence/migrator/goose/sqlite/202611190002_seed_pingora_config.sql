-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('pingora_enabled', 'false', 'business', 0, 'Pingora 独立发布组开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'pingora_enabled';
