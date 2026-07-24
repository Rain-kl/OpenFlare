-- +goose Up
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS cache_status String DEFAULT '';

-- +goose Down
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS cache_status;
