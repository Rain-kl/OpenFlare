-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES 
  ('login_session_ttl_hours', '0', 'system', 0, '登录会话过期时间 (小时，0表示浏览器关闭后自动退出，-1表示永不过期)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'login_session_ttl_hours';
