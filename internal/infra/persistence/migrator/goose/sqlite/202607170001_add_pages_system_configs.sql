-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('pages_max_package_size_mb', '100', 'business', 0, 'Pages 部署包上传大小上限（MiB）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('pages_max_history_count', '20', 'business', 0, 'Pages 每个项目最大历史部署保留数（0 表示不限制）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN ('pages_max_package_size_mb', 'pages_max_history_count');
