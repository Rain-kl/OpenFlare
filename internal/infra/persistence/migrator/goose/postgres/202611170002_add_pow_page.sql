-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('pow_page', '{"template":"classic","widget":"progress"}', 'business', 0, 'PoW 页面模板、组件样式与自定义 HTML', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'pow_page';
