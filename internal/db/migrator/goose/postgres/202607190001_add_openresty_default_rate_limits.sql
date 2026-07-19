-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('openresty_default_limit_conn_per_server', '0', 'business', 0, '默认站点并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('openresty_default_limit_conn_per_ip', '0', 'business', 0, '默认单 IP 并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('openresty_default_limit_rate', '', 'business', 0, '默认单请求带宽限速（空关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'openresty_default_limit_conn_per_server',
  'openresty_default_limit_conn_per_ip',
  'openresty_default_limit_rate'
);
