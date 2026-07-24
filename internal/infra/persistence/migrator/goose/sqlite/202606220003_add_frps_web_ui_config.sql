-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES 
  ('relay_frps_web_ui_enabled', 'false', 'business', 0, '是否启用 FRPS 内置 Web 管理界面', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('relay_frps_web_ui_port', '17500', 'business', 0, 'FRPS 内置 Web 管理界面端口', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN ('relay_frps_web_ui_enabled', 'relay_frps_web_ui_port');
