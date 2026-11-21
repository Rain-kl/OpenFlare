-- +goose Up
UPDATE w_system_configs
SET description = '客户端可访问的 HTTP 或 HTTPS 地址，留空表示禁用 CLI 与 MCP。'
WHERE key = 'automation_public_url';

-- +goose Down
UPDATE w_system_configs
SET description = '自动化对外 HTTPS 地址，留空表示禁用 CLI 与 MCP。'
WHERE key = 'automation_public_url';
