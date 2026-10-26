-- +goose Up
CREATE INDEX IF NOT EXISTS idx_of_nodes_type_id ON of_nodes (node_type, id);
CREATE INDEX IF NOT EXISTS idx_of_nodes_last_seen_id ON of_nodes (last_seen_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_of_nodes_last_seen_id;
DROP INDEX IF EXISTS idx_of_nodes_type_id;
