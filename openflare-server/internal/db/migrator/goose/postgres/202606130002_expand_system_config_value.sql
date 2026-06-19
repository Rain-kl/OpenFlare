-- +goose Up
ALTER TABLE w_system_configs ALTER COLUMN value TYPE TEXT;

-- +goose Down
ALTER TABLE w_system_configs ALTER COLUMN value TYPE VARCHAR(255);
