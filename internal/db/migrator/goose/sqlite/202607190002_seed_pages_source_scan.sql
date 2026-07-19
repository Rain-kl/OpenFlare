-- +goose Up
INSERT INTO w_schedules (name, task_type, cron, payload, is_active, created_at, updated_at)
SELECT
    'OpenFlare Pages 部署源扫描',
    'of_pages_source_scan',
    '*/5 * * * *',
    '{}',
    1,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM w_schedules WHERE task_type = 'of_pages_source_scan'
);

-- +goose Down
DELETE FROM w_schedules
WHERE task_type = 'of_pages_source_scan'
  AND name = 'OpenFlare Pages 部署源扫描'
  AND cron = '*/5 * * * *'
  AND payload = '{}'
  AND is_active = 1;
