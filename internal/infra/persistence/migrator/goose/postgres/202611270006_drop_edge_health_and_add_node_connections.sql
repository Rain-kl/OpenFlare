-- +goose Up
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS openresty_connections BIGINT NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS of_node_edge_health CASCADE;

-- +goose Down
CREATE TABLE IF NOT EXISTS of_node_edge_health (
    id          BIGINT NOT NULL PRIMARY KEY,
    node_id     VARCHAR(64) NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NOT NULL,
    status      VARCHAR(64) NOT NULL DEFAULT '',
    connections BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);
ALTER TABLE of_nodes DROP COLUMN IF EXISTS openresty_connections;
