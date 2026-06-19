-- +goose Up
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_node_id_logged_at ON of_node_access_logs (node_id, logged_at);

-- +goose Down
DROP INDEX IF EXISTS idx_of_node_access_logs_node_id_logged_at;