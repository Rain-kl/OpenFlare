-- +goose Up
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS user_agent String DEFAULT '';

-- +goose Down
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS user_agent;
