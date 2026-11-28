-- +goose Up
INSERT INTO w_schedules (name, task_type, cron, payload, is_active, hide_success_log, created_at, updated_at)
SELECT 'OpenFlare DNS 账号作用域刷新', 'of_dns_account_refresh', '17 3 * * *', '{}', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM w_schedules WHERE task_type = 'of_dns_account_refresh');

-- +goose Down
DELETE FROM w_schedules WHERE task_type = 'of_dns_account_refresh' AND name = 'OpenFlare DNS 账号作用域刷新';
