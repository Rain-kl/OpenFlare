-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('update_upstream_repository', 'Rain-kl/OpenFlare', 'system', 0, 'GitHub Actions Release 上游仓库（owner/repo 或 GitHub 仓库地址）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'update_upstream_repository';
