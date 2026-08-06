-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('origin_error_page_get_only', 'false', 'business', 0, '源站错误页是否仅对 GET 请求生效（其它方法透传）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'origin_error_page_get_only';
