-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('file_access_whitelist', '["avatar"]', 'system', 1, '免登录访问的文件业务类型白名单 (JSON 数组格式)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'file_access_whitelist';
