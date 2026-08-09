-- +goose Up
-- 系统定期垃圾清理改为每日执行一次（凌晨 3 点，Asia/Shanghai）。
-- 此前为每 2 小时（0 */2 * * *）高频扫描，日常清理收益有限，降频减少非必要扫描。
UPDATE w_schedules
SET cron = '0 3 * * *',
    updated_at = CURRENT_TIMESTAMP
WHERE task_type = 'system_cleanup';

-- +goose Down
UPDATE w_schedules
SET cron = '0 */2 * * *',
    updated_at = CURRENT_TIMESTAMP
WHERE task_type = 'system_cleanup';
