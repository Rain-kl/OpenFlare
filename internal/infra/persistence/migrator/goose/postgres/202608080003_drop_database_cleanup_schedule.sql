-- +goose Up
DELETE FROM w_schedules WHERE task_type = 'of_database_auto_cleanup';

-- +goose Down
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (102, 'OpenFlare 可观测数据自动清理', 'of_database_auto_cleanup', '0 3 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
