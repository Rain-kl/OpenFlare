-- +goose Up
ALTER TABLE access_tokens ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE access_tokens DROP COLUMN is_admin;
