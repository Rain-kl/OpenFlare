-- +goose Up
ALTER TABLE w_schedules ADD COLUMN hide_success_log BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE w_schedules DROP COLUMN hide_success_log;
