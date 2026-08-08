-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('sw_offline_enabled', 'false', 'business', 0, '是否启用 Service Worker 离线兜底', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('sw_offline_html', '', 'business', 0, '离线联系页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'sw_offline_enabled',
  'sw_offline_html'
);
