-- +goose Up
-- Earlier built-in schedules used explicit IDs, so advance the identity only
-- when it trails either existing rows or an already-higher sequence value.
SELECT setval(
    pg_get_serial_sequence('w_schedules', 'id'),
    GREATEST(
        1,
        COALESCE((SELECT MAX(id) FROM w_schedules), 0),
        COALESCE((
            SELECT sequences.last_value
            FROM pg_sequences AS sequences
            WHERE format('%I.%I', sequences.schemaname, sequences.sequencename)::regclass =
                  pg_get_serial_sequence('w_schedules', 'id')::regclass
        ), 0)
    ),
    TRUE
);

INSERT INTO w_schedules (name, task_type, cron, payload, is_active, created_at, updated_at)
SELECT
    'OpenFlare Pages 部署源扫描',
    'of_pages_source_scan',
    '*/5 * * * *',
    '{}',
    TRUE,
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
  AND is_active = TRUE;
