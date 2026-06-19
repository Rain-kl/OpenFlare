-- +goose Up
ALTER TABLE access_tokens ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE access_tokens DROP COLUMN IF EXISTS is_admin;
