-- +goose Up
ALTER TABLE w_uploads ADD COLUMN access_mode INTEGER NOT NULL DEFAULT 0;
UPDATE w_uploads SET access_mode = 1 WHERE type = 'avatar';

-- +goose Down
ALTER TABLE w_uploads DROP COLUMN access_mode;
