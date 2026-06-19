-- +goose Up
CREATE TABLE of_node_obs_frpc (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    tunnel_status VARCHAR(16) NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_obs_frpc_node_id ON of_node_obs_frpc (node_id);
CREATE INDEX idx_of_node_obs_frpc_captured_at ON of_node_obs_frpc (captured_at);

-- +goose Down
DROP TABLE IF EXISTS of_node_obs_frpc;