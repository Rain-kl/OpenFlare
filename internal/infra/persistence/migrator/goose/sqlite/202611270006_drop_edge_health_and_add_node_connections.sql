-- +goose Up
ALTER TABLE of_nodes ADD COLUMN openresty_connections INTEGER NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS of_node_edge_health;

-- +goose Down
CREATE TABLE IF NOT EXISTS of_node_edge_health (
    id          INTEGER NOT NULL PRIMARY KEY,
    node_id     TEXT NOT NULL DEFAULT '',
    captured_at DATETIME NOT NULL,
    status      TEXT NOT NULL DEFAULT '',
    connections INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);
ALTER TABLE of_nodes DROP COLUMN openresty_connections;
