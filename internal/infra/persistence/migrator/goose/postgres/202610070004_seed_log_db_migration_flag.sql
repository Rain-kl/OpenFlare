-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('log_db_migration', '', 'system', 0, '日志数据库切换冻结标记', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
-- Keep the runtime migration flag: this row may have existed before this migration.
