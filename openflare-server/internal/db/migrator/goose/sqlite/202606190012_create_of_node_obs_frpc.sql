-- +goose Up
CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    tunnel_status TEXT NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node_id ON of_node_obs_frpc (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_captured_at ON of_node_obs_frpc (captured_at);

-- +goose Down
DROP TABLE IF EXISTS of_node_obs_frpc;