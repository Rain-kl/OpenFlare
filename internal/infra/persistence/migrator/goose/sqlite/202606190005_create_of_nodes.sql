-- +goose Up
CREATE TABLE IF NOT EXISTS of_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ip TEXT NOT NULL DEFAULT '',
    ip_manual_override INTEGER NOT NULL DEFAULT 0,
    geo_name TEXT NOT NULL DEFAULT '',
    geo_latitude REAL,
    geo_longitude REAL,
    geo_manual_override INTEGER NOT NULL DEFAULT 0,
    access_token TEXT NOT NULL DEFAULT '',
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    update_requested INTEGER NOT NULL DEFAULT 0,
    update_channel TEXT NOT NULL DEFAULT 'stable',
    update_tag TEXT NOT NULL DEFAULT '',
    restart_openresty_requested INTEGER NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT '',
    ext_version TEXT NOT NULL DEFAULT '',
    openresty_status TEXT NOT NULL DEFAULT 'unknown',
    openresty_message TEXT,
    status TEXT NOT NULL DEFAULT 'offline',
    current_version TEXT NOT NULL DEFAULT '',
    last_seen_at DATETIME,
    last_error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    node_type TEXT NOT NULL DEFAULT 'edge_node',
    relay_bind_port INTEGER NOT NULL DEFAULT 0,
    relay_vhost_http_port INTEGER NOT NULL DEFAULT 0,
    relay_auth_token TEXT NOT NULL DEFAULT '',
    relay_agent_access_addr TEXT NOT NULL DEFAULT '',
    relay_client_access_addr TEXT NOT NULL DEFAULT '',
    relay_client_proxy_url TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    relay_status TEXT NOT NULL DEFAULT 'unknown',
    relay_web_server_enabled INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_nodes_node_id ON of_nodes (node_id);
CREATE INDEX IF NOT EXISTS idx_of_nodes_access_token ON of_nodes (access_token);

-- +goose Down
DROP TABLE IF EXISTS of_nodes;