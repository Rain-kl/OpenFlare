-- +goose Up
-- 性能指标（CPU/内存/磁盘/网络）保留天数：三库共用、独立短留存（默认 3 天），
-- 不随 log_retention_days_*（访问日志保留配置）变化。
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('metric_retention_days', '3', 'business', 0, '性能指标（CPU/内存/磁盘/网络）保留天数（三库共用）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'metric_retention_days';
