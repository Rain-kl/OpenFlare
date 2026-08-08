-- +goose Up
CREATE TABLE IF NOT EXISTS w_schedules_backup_of_database_auto_cleanup AS
SELECT * FROM w_schedules WHERE task_type = 'of_database_auto_cleanup';

DELETE FROM w_schedules WHERE task_type = 'of_database_auto_cleanup';

-- +goose Down
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
SELECT id, name, task_type, cron, payload, is_active, created_at, updated_at
FROM w_schedules_backup_of_database_auto_cleanup
ON CONFLICT (id) DO NOTHING;

INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
SELECT 102, 'OpenFlare 可观测数据自动清理', 'of_database_auto_cleanup', '0 3 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM w_schedules WHERE task_type = 'of_database_auto_cleanup')
ON CONFLICT (id) DO NOTHING;

DROP TABLE IF EXISTS w_schedules_backup_of_database_auto_cleanup;

