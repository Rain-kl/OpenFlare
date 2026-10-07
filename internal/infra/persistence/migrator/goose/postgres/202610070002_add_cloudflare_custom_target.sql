-- +goose Up
ALTER TABLE of_cf_pointing_groups
    ADD COLUMN target_mode VARCHAR(16) NOT NULL DEFAULT 'node',
    ADD COLUMN record_type VARCHAR(8) NOT NULL DEFAULT 'A',
    ADD COLUMN record_content VARCHAR(255) NOT NULL DEFAULT '';

INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (105, 'Cloudflare 节点故障回退检查', 'of_cloudflare_failover_check', '* * * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM w_schedules WHERE id = 105;
ALTER TABLE of_cf_pointing_groups
    DROP COLUMN record_content,
    DROP COLUMN record_type,
    DROP COLUMN target_mode;
