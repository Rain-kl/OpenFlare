-- +goose Up
CREATE TABLE IF NOT EXISTS of_nodes (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    ip VARCHAR(64) NOT NULL DEFAULT '',
    ip_manual_override BOOLEAN NOT NULL DEFAULT FALSE,
    geo_name VARCHAR(128) NOT NULL DEFAULT '',
    geo_latitude DOUBLE PRECISION,
    geo_longitude DOUBLE PRECISION,
    geo_manual_override BOOLEAN NOT NULL DEFAULT FALSE,
    access_token VARCHAR(128) NOT NULL DEFAULT '',
    auto_update_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    update_requested BOOLEAN NOT NULL DEFAULT FALSE,
    update_channel VARCHAR(16) NOT NULL DEFAULT 'stable',
    update_tag VARCHAR(64) NOT NULL DEFAULT '',
    restart_openresty_requested BOOLEAN NOT NULL DEFAULT FALSE,
    version VARCHAR(64) NOT NULL DEFAULT '',
    ext_version VARCHAR(64) NOT NULL DEFAULT '',
    openresty_status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    openresty_message TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'offline',
    current_version VARCHAR(32) NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    node_type VARCHAR(32) NOT NULL DEFAULT 'edge_node',
    relay_bind_port INTEGER NOT NULL DEFAULT 0,
    relay_vhost_http_port INTEGER NOT NULL DEFAULT 0,
    relay_auth_token VARCHAR(128) NOT NULL DEFAULT '',
    relay_agent_access_addr VARCHAR(255) NOT NULL DEFAULT '',
    relay_client_access_addr VARCHAR(255) NOT NULL DEFAULT '',
    relay_client_proxy_url VARCHAR(512) NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    relay_status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    relay_web_server_enabled BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_nodes_node_id ON of_nodes (node_id);
CREATE INDEX IF NOT EXISTS idx_of_nodes_access_token ON of_nodes (access_token);

-- +goose Down
DROP TABLE IF EXISTS of_nodes;