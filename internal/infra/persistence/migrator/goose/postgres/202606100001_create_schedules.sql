-- +goose Up
CREATE TABLE IF NOT EXISTS schedules (
    id BIGINT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    task_type VARCHAR(64) NOT NULL,
    cron VARCHAR(64) NOT NULL,
    payload TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_schedules_is_active ON schedules (is_active);

-- Seed initial cleanup task
INSERT INTO schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (1, '清理未使用上传', 'cleanup_unused_uploads', '0 */2 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS schedules;
