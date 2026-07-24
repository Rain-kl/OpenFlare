-- +goose Up
ALTER TABLE w_push_events ADD COLUMN task_type VARCHAR(100) NOT NULL DEFAULT '';
CREATE INDEX idx_w_push_events_task_type ON w_push_events(task_type);

-- +goose Down
DROP INDEX IF EXISTS idx_w_push_events_task_type;
-- SQLite does not support DROP COLUMN in older versions easily, but standard ALTER TABLE DROP COLUMN works in SQLite 3.35.0+.
-- We can write standard DROP COLUMN.
ALTER TABLE w_push_events DROP COLUMN task_type;
