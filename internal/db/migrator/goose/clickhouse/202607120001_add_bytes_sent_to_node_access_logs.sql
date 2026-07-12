-- +goose Up
ALTER TABLE of_node_access_logs ADD COLUMN IF NOT EXISTS bytes_sent UInt64;

-- +goose Down
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS bytes_sent;
