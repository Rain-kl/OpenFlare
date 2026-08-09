-- +goose Up
-- 日志保留天数配置（business），替换旧的 database_auto_cleanup_* 键。
-- 默认统一 30 天：不继承旧键值，避免旧配置被静默带入导致日志被过度清理。
INSERT OR IGNORE INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
SELECT 'log_retention_days_postgres', '30', 'business', 0, 'PostgreSQL 日志保留天数（访问日志与可观测统一）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
UNION ALL
SELECT 'log_retention_days_sqlite', '30', 'business', 0, 'SQLite 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
UNION ALL
SELECT 'log_retention_days_clickhouse', '30', 'business', 0, 'ClickHouse 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP;

DELETE FROM w_system_configs WHERE key IN ('database_auto_cleanup_enabled', 'database_auto_cleanup_retention_days');

-- +goose Down
INSERT OR IGNORE INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
SELECT 'database_auto_cleanup_enabled', 'true', 'business', 0, '数据库自动清理开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
UNION ALL
SELECT 'database_auto_cleanup_retention_days', COALESCE((SELECT value FROM w_system_configs WHERE key = 'log_retention_days_sqlite'), '30'), 'business', 0, '数据库保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP;

DELETE FROM w_system_configs WHERE key IN ('log_retention_days_postgres', 'log_retention_days_sqlite', 'log_retention_days_clickhouse');

