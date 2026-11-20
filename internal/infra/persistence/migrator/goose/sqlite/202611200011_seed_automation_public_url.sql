-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description)
VALUES ('automation_public_url', '', 'system', 0, '自动化对外 HTTPS 地址，留空表示禁用 CLI 与 MCP。')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'automation_public_url';
