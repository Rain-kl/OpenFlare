-- +goose Up
ALTER TABLE of_cf_pointing_groups ADD COLUMN target_mode VARCHAR(16) NOT NULL DEFAULT 'node';
ALTER TABLE of_cf_pointing_groups ADD COLUMN record_type VARCHAR(8) NOT NULL DEFAULT 'A';
ALTER TABLE of_cf_pointing_groups ADD COLUMN record_content VARCHAR(255) NOT NULL DEFAULT '';

INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (105, 'Cloudflare 节点故障回退检查', 'of_cloudflare_failover_check', '* * * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM w_schedules WHERE id = 105;
ALTER TABLE of_cf_pointing_groups DROP COLUMN record_content;
ALTER TABLE of_cf_pointing_groups DROP COLUMN record_type;
ALTER TABLE of_cf_pointing_groups DROP COLUMN target_mode;
