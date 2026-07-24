-- +goose Up
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES
    (101, 'OpenFlare SSL 自动续期', 'of_ssl_renew', '0 0 * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (102, 'OpenFlare 可观测数据自动清理', 'of_database_auto_cleanup', '0 3 * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (103, 'OpenFlare WAF IP 组同步', 'of_waf_ip_group_sync', '*/5 * * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (104, 'OpenFlare Uptime Kuma 同步', 'of_uptime_kuma_sync', '* * * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM w_schedules WHERE id IN (101, 102, 103, 104);