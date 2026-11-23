-- +goose Up
DROP TRIGGER IF EXISTS dirty_edge_relay_insert;
DROP TRIGGER IF EXISTS dirty_edge_relay_update;
DROP TRIGGER IF EXISTS dirty_edge_relay_delete;

ALTER TABLE of_nodes DROP COLUMN relay_bind_port;
ALTER TABLE of_nodes DROP COLUMN relay_vhost_http_port;
ALTER TABLE of_nodes DROP COLUMN relay_auth_token;
ALTER TABLE of_nodes DROP COLUMN relay_agent_access_addr;
ALTER TABLE of_nodes DROP COLUMN relay_client_access_addr;
ALTER TABLE of_nodes DROP COLUMN relay_client_proxy_url;
ALTER TABLE of_nodes DROP COLUMN relay_status;
ALTER TABLE of_nodes DROP COLUMN relay_web_server_enabled;

DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_frpc;

DROP INDEX IF EXISTS idx_of_proxy_routes_tunnel_node_id;
ALTER TABLE of_proxy_routes DROP COLUMN tunnel_node_id;
ALTER TABLE of_proxy_routes ADD COLUMN tunnel_id INTEGER;
ALTER TABLE of_proxy_routes ADD COLUMN tunnel_options_json TEXT NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_id ON of_proxy_routes (tunnel_id);

CREATE TABLE IF NOT EXISTS of_tunnel_relay_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    is_default INTEGER NOT NULL DEFAULT 0,
    routing_policy_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_relay_pools_pool_key ON of_tunnel_relay_pools (pool_key);

CREATE TABLE IF NOT EXISTS of_tunnel_relays (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    relay_id TEXT NOT NULL,
    pool_id INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    access_token TEXT NOT NULL DEFAULT '',
    public_addr TEXT NOT NULL DEFAULT '',
    ip_manual_override INTEGER NOT NULL DEFAULT 0,
    transport_port INTEGER NOT NULL DEFAULT 7443,
    ws_port INTEGER NOT NULL DEFAULT 0,
    gateway_addr TEXT NOT NULL DEFAULT '',
    gateway_port INTEGER NOT NULL DEFAULT 8443,
    gateway_tls INTEGER NOT NULL DEFAULT 0,
    gateway_secret TEXT NOT NULL DEFAULT '',
    supported_transports_json TEXT NOT NULL DEFAULT '["noise","tcp"]',
    noise_private_key TEXT NOT NULL DEFAULT '',
    noise_public_key TEXT NOT NULL DEFAULT '',
    geo_name TEXT NOT NULL DEFAULT '',
    geo_latitude REAL,
    geo_longitude REAL,
    geo_manual_override INTEGER NOT NULL DEFAULT 0,
    zone_tag TEXT NOT NULL DEFAULT '',
    colocated_edge_node_id TEXT NOT NULL DEFAULT '',
    weight INTEGER NOT NULL DEFAULT 100,
    max_tunnels INTEGER NOT NULL DEFAULT 0,
    draining INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    runtime_status TEXT NOT NULL DEFAULT 'unknown',
    active_connectors INTEGER NOT NULL DEFAULT 0,
    active_streams INTEGER NOT NULL DEFAULT 0,
    cpu_usage_percent REAL NOT NULL DEFAULT 0,
    memory_usage_percent REAL NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT '',
    engine_version TEXT NOT NULL DEFAULT '',
    contract_version INTEGER NOT NULL DEFAULT 0,
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    update_requested INTEGER NOT NULL DEFAULT 0,
    update_channel TEXT NOT NULL DEFAULT 'stable',
    update_tag TEXT NOT NULL DEFAULT '',
    last_seen_at DATETIME,
    last_error TEXT,
    quarantined_at DATETIME,
    quarantine_reason TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_relays_relay_id ON of_tunnel_relays (relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_pool_id ON of_tunnel_relays (pool_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_access_token ON of_tunnel_relays (access_token);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_colocated_edge_node_id ON of_tunnel_relays (colocated_edge_node_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_status ON of_tunnel_relays (status);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_last_seen_at ON of_tunnel_relays (last_seen_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_quarantined_at ON of_tunnel_relays (quarantined_at);

CREATE TABLE IF NOT EXISTS of_tunnels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tunnel_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    pool_id INTEGER NOT NULL DEFAULT 0,
    access_token TEXT NOT NULL DEFAULT '',
    ha_relay_count INTEGER NOT NULL DEFAULT 2,
    preferred_transport TEXT NOT NULL DEFAULT 'auto',
    fallback_transports_json TEXT NOT NULL DEFAULT '["noise","websocket","tcp"]',
    client_proxy_url TEXT NOT NULL DEFAULT '',
    allowed_target_cidrs_json TEXT NOT NULL DEFAULT '[]',
    blocked_target_cidrs_json TEXT NOT NULL DEFAULT '["169.254.169.254/32"]',
    max_connectors INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    config_revision INTEGER NOT NULL DEFAULT 1,
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    update_requested INTEGER NOT NULL DEFAULT 0,
    update_channel TEXT NOT NULL DEFAULT 'stable',
    update_tag TEXT NOT NULL DEFAULT '',
    quarantined_at DATETIME,
    quarantine_reason TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnels_tunnel_id ON of_tunnels (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_pool_id ON of_tunnels (pool_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_access_token ON of_tunnels (access_token);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_quarantined_at ON of_tunnels (quarantined_at);

CREATE TABLE IF NOT EXISTS of_tunnel_connectors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    connector_id TEXT NOT NULL,
    tunnel_id INTEGER NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    os_arch TEXT NOT NULL DEFAULT '',
    public_ip TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    engine_version TEXT NOT NULL DEFAULT '',
    contract_version INTEGER NOT NULL DEFAULT 0,
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    applied_revision INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'online',
    last_error TEXT,
    connected_relays_json TEXT NOT NULL DEFAULT '[]',
    draining INTEGER NOT NULL DEFAULT 0,
    revoked_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_connectors_connector_id ON of_tunnel_connectors (connector_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_tunnel_id ON of_tunnel_connectors (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_status ON of_tunnel_connectors (status);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_last_seen_at ON of_tunnel_connectors (last_seen_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_tunnel_seen ON of_tunnel_connectors (tunnel_id, last_seen_at);

CREATE TABLE IF NOT EXISTS of_tunnel_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tunnel_id INTEGER NOT NULL,
    connector_id TEXT NOT NULL,
    relay_id INTEGER NOT NULL,
    service_name TEXT NOT NULL DEFAULT '',
    session_token TEXT NOT NULL DEFAULT '',
    transport TEXT NOT NULL DEFAULT 'noise',
    state TEXT NOT NULL DEFAULT 'connecting',
    rtt_ms INTEGER NOT NULL DEFAULT 0,
    active_streams INTEGER NOT NULL DEFAULT 0,
    relay_verified_at DATETIME,
    lease_expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_attachments_conn_relay ON of_tunnel_attachments (connector_id, relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_tunnel_id ON of_tunnel_attachments (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_connector_id ON of_tunnel_attachments (connector_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_relay_id ON of_tunnel_attachments (relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_state ON of_tunnel_attachments (state);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_lease_expires_at ON of_tunnel_attachments (lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_routing ON of_tunnel_attachments (tunnel_id, state, lease_expires_at);

CREATE TABLE IF NOT EXISTS of_tunnel_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tunnel_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    service_type TEXT NOT NULL DEFAULT 'tcp',
    relay_id INTEGER NOT NULL DEFAULT 0,
    listen_port INTEGER NOT NULL DEFAULT 0,
    target_addr TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    options_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_services_tunnel_id ON of_tunnel_services (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_services_relay_id ON of_tunnel_services (relay_id);

-- +goose Down
DROP TABLE IF EXISTS of_tunnel_services;
DROP TABLE IF EXISTS of_tunnel_attachments;
DROP TABLE IF EXISTS of_tunnel_connectors;
DROP TABLE IF EXISTS of_tunnels;
DROP TABLE IF EXISTS of_tunnel_relays;
DROP TABLE IF EXISTS of_tunnel_relay_pools;

DROP INDEX IF EXISTS idx_of_proxy_routes_tunnel_id;
ALTER TABLE of_proxy_routes DROP COLUMN tunnel_options_json;
ALTER TABLE of_proxy_routes DROP COLUMN tunnel_id;
ALTER TABLE of_proxy_routes ADD COLUMN tunnel_node_id INTEGER;
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL DEFAULT '',
    captured_at DATETIME NOT NULL,
    frps_connections INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL DEFAULT '',
    captured_at DATETIME NOT NULL,
    tunnel_status TEXT NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

ALTER TABLE of_nodes ADD COLUMN relay_bind_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN relay_vhost_http_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN relay_auth_token TEXT NOT NULL DEFAULT '';
ALTER TABLE of_nodes ADD COLUMN relay_agent_access_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE of_nodes ADD COLUMN relay_client_access_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE of_nodes ADD COLUMN relay_client_proxy_url TEXT NOT NULL DEFAULT '';
ALTER TABLE of_nodes ADD COLUMN relay_status TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE of_nodes ADD COLUMN relay_web_server_enabled INTEGER NOT NULL DEFAULT 0;

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_insert AFTER INSERT ON of_nodes
WHEN NEW.node_type = 'tunnel_relay'
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_update AFTER UPDATE OF node_type, ip, relay_vhost_http_port, relay_agent_access_addr, relay_client_access_addr ON of_nodes
WHEN (NEW.node_type = 'tunnel_relay' OR OLD.node_type = 'tunnel_relay') AND (NEW.node_type IS NOT OLD.node_type OR NEW.ip IS NOT OLD.ip OR NEW.relay_vhost_http_port IS NOT OLD.relay_vhost_http_port OR NEW.relay_agent_access_addr IS NOT OLD.relay_agent_access_addr OR NEW.relay_client_access_addr IS NOT OLD.relay_client_access_addr)
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_delete AFTER DELETE ON of_nodes
WHEN OLD.node_type = 'tunnel_relay'
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd
