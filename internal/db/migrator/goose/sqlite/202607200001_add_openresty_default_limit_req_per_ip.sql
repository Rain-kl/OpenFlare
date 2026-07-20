-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('openresty_default_limit_req_per_ip', '', 'business', 0, '默认单 IP 请求频率限制（空关闭，例如 10r/s、100r/m）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

ALTER TABLE of_proxy_routes ADD COLUMN limit_req_per_ip VARCHAR(32) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE of_proxy_routes DROP COLUMN limit_req_per_ip;

DELETE FROM w_system_configs WHERE key = 'openresty_default_limit_req_per_ip';
