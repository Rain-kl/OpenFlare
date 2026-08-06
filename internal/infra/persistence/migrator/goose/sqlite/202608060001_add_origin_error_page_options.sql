-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('origin_error_page_enabled', 'true', 'business', 0, '是否启用源站错误页', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('origin_error_page_status_codes', '["500-599"]', 'business', 0, '源站错误页触发状态码标签 JSON 数组', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('origin_error_page_html', '', 'business', 0, '源站错误页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'origin_error_page_enabled',
  'origin_error_page_status_codes',
  'origin_error_page_html'
);
