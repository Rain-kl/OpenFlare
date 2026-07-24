-- +goose Up
UPDATE w_schedules
SET name = '系统定期垃圾清理',
    task_type = 'system_cleanup'
WHERE id = 1;

-- +goose Down
UPDATE w_schedules
SET name = '清理未使用上传',
    task_type = 'cleanup_unused_uploads'
WHERE id = 1;
