-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('sw_offline_domains', '[]', 'business', 0, 'SW 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'sw_offline_domains';
