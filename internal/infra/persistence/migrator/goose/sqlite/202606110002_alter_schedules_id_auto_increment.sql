-- +goose Up
-- +goose StatementBegin
-- 1. Rename existing schedules table
ALTER TABLE schedules RENAME TO schedules_old;

-- 2. Create new schedules table with AUTOINCREMENT
CREATE TABLE schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(128) NOT NULL,
    task_type VARCHAR(64) NOT NULL,
    cron VARCHAR(64) NOT NULL,
    payload TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 3. Copy existing data
INSERT INTO schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
SELECT id, name, task_type, cron, payload, is_active, created_at, updated_at FROM schedules_old;

-- 4. Drop the old table
DROP TABLE schedules_old;

-- 5. Recreate index
CREATE INDEX IF NOT EXISTS idx_schedules_is_active ON schedules (is_active);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Re-creating table with AUTOINCREMENT cannot be undone simply without recreating table again.
-- +goose StatementEnd
