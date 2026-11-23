-- +goose Up
DROP TRIGGER IF EXISTS dirty_edge_relay_insert ON of_nodes;
DROP TRIGGER IF EXISTS dirty_edge_relay_update ON of_nodes;
DROP TRIGGER IF EXISTS dirty_edge_relay_delete ON of_nodes;

ALTER TABLE of_nodes
    DROP COLUMN IF EXISTS relay_bind_port,
    DROP COLUMN IF EXISTS relay_vhost_http_port,
    DROP COLUMN IF EXISTS relay_auth_token,
    DROP COLUMN IF EXISTS relay_agent_access_addr,
    DROP COLUMN IF EXISTS relay_client_access_addr,
    DROP COLUMN IF EXISTS relay_client_proxy_url,
    DROP COLUMN IF EXISTS relay_status,
    DROP COLUMN IF EXISTS relay_web_server_enabled;

DROP TABLE IF EXISTS of_node_obs_frps CASCADE;
DROP TABLE IF EXISTS of_node_obs_frpc CASCADE;

DROP INDEX IF EXISTS idx_of_proxy_routes_tunnel_node_id;
ALTER TABLE of_proxy_routes
    DROP COLUMN IF EXISTS tunnel_node_id,
    ADD COLUMN IF NOT EXISTS tunnel_id BIGINT,
    ADD COLUMN IF NOT EXISTS tunnel_options_json TEXT NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_id ON of_proxy_routes (tunnel_id);

CREATE TABLE IF NOT EXISTS of_tunnel_relay_pools (
    id BIGSERIAL PRIMARY KEY,
    pool_key VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    region VARCHAR(64) NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    routing_policy_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_relay_pools_pool_key ON of_tunnel_relay_pools (pool_key);

CREATE TABLE IF NOT EXISTS of_tunnel_relays (
    id BIGSERIAL PRIMARY KEY,
    relay_id VARCHAR(64) NOT NULL,
    pool_id BIGINT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL,
    access_token VARCHAR(128) NOT NULL DEFAULT '',
    public_addr VARCHAR(255) NOT NULL DEFAULT '',
    ip_manual_override BOOLEAN NOT NULL DEFAULT FALSE,
    transport_port INTEGER NOT NULL DEFAULT 7443,
    ws_port INTEGER NOT NULL DEFAULT 0,
    gateway_addr VARCHAR(255) NOT NULL DEFAULT '',
    gateway_port INTEGER NOT NULL DEFAULT 8443,
    gateway_tls BOOLEAN NOT NULL DEFAULT FALSE,
    gateway_secret VARCHAR(128) NOT NULL DEFAULT '',
    supported_transports_json TEXT NOT NULL DEFAULT '["noise","tcp"]',
    noise_private_key VARCHAR(128) NOT NULL DEFAULT '',
    noise_public_key VARCHAR(128) NOT NULL DEFAULT '',
    geo_name VARCHAR(128) NOT NULL DEFAULT '',
    geo_latitude DOUBLE PRECISION,
    geo_longitude DOUBLE PRECISION,
    geo_manual_override BOOLEAN NOT NULL DEFAULT FALSE,
    zone_tag VARCHAR(64) NOT NULL DEFAULT '',
    colocated_edge_node_id VARCHAR(64) NOT NULL DEFAULT '',
    weight INTEGER NOT NULL DEFAULT 100,
    max_tunnels INTEGER NOT NULL DEFAULT 0,
    draining BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    runtime_status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    active_connectors INTEGER NOT NULL DEFAULT 0,
    active_streams INTEGER NOT NULL DEFAULT 0,
    cpu_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    version VARCHAR(64) NOT NULL DEFAULT '',
    engine_version VARCHAR(64) NOT NULL DEFAULT '',
    contract_version INTEGER NOT NULL DEFAULT 0,
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    auto_update_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    update_requested BOOLEAN NOT NULL DEFAULT FALSE,
    update_channel VARCHAR(16) NOT NULL DEFAULT 'stable',
    update_tag VARCHAR(64) NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ,
    last_error TEXT,
    quarantined_at TIMESTAMPTZ,
    quarantine_reason VARCHAR(255) NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_relays_relay_id ON of_tunnel_relays (relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_pool_id ON of_tunnel_relays (pool_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_access_token ON of_tunnel_relays (access_token);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_colocated_edge_node_id ON of_tunnel_relays (colocated_edge_node_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_status ON of_tunnel_relays (status);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_last_seen_at ON of_tunnel_relays (last_seen_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_relays_quarantined_at ON of_tunnel_relays (quarantined_at);

CREATE TABLE IF NOT EXISTS of_tunnels (
    id BIGSERIAL PRIMARY KEY,
    tunnel_id VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    pool_id BIGINT NOT NULL DEFAULT 0,
    access_token VARCHAR(128) NOT NULL DEFAULT '',
    ha_relay_count INTEGER NOT NULL DEFAULT 2,
    preferred_transport VARCHAR(16) NOT NULL DEFAULT 'auto',
    fallback_transports_json TEXT NOT NULL DEFAULT '["noise","websocket","tcp"]',
    client_proxy_url VARCHAR(512) NOT NULL DEFAULT '',
    allowed_target_cidrs_json TEXT NOT NULL DEFAULT '[]',
    blocked_target_cidrs_json TEXT NOT NULL DEFAULT '["169.254.169.254/32"]',
    max_connectors INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config_revision BIGINT NOT NULL DEFAULT 1,
    auto_update_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    update_requested BOOLEAN NOT NULL DEFAULT FALSE,
    update_channel VARCHAR(16) NOT NULL DEFAULT 'stable',
    update_tag VARCHAR(64) NOT NULL DEFAULT '',
    quarantined_at TIMESTAMPTZ,
    quarantine_reason VARCHAR(255) NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnels_tunnel_id ON of_tunnels (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_pool_id ON of_tunnels (pool_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_access_token ON of_tunnels (access_token);
CREATE INDEX IF NOT EXISTS idx_of_tunnels_quarantined_at ON of_tunnels (quarantined_at);

CREATE TABLE IF NOT EXISTS of_tunnel_connectors (
    id BIGSERIAL PRIMARY KEY,
    connector_id VARCHAR(64) NOT NULL,
    tunnel_id BIGINT NOT NULL,
    hostname VARCHAR(255) NOT NULL DEFAULT '',
    os_arch VARCHAR(64) NOT NULL DEFAULT '',
    public_ip VARCHAR(64) NOT NULL DEFAULT '',
    version VARCHAR(64) NOT NULL DEFAULT '',
    engine_version VARCHAR(64) NOT NULL DEFAULT '',
    contract_version INTEGER NOT NULL DEFAULT 0,
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    applied_revision BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'online',
    last_error TEXT,
    connected_relays_json TEXT NOT NULL DEFAULT '[]',
    draining BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_connectors_connector_id ON of_tunnel_connectors (connector_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_tunnel_id ON of_tunnel_connectors (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_status ON of_tunnel_connectors (status);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_last_seen_at ON of_tunnel_connectors (last_seen_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_connectors_tunnel_seen ON of_tunnel_connectors (tunnel_id, last_seen_at);

CREATE TABLE IF NOT EXISTS of_tunnel_attachments (
    id BIGSERIAL PRIMARY KEY,
    tunnel_id BIGINT NOT NULL,
    connector_id VARCHAR(64) NOT NULL,
    relay_id BIGINT NOT NULL,
    service_name VARCHAR(128) NOT NULL DEFAULT '',
    session_token VARCHAR(128) NOT NULL DEFAULT '',
    transport VARCHAR(16) NOT NULL DEFAULT 'noise',
    state VARCHAR(16) NOT NULL DEFAULT 'connecting',
    rtt_ms INTEGER NOT NULL DEFAULT 0,
    active_streams INTEGER NOT NULL DEFAULT 0,
    relay_verified_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tunnel_attachments_conn_relay ON of_tunnel_attachments (connector_id, relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_tunnel_id ON of_tunnel_attachments (tunnel_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_connector_id ON of_tunnel_attachments (connector_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_relay_id ON of_tunnel_attachments (relay_id);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_state ON of_tunnel_attachments (state);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_lease_expires_at ON of_tunnel_attachments (lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_of_tunnel_attachments_routing ON of_tunnel_attachments (tunnel_id, state, lease_expires_at);

CREATE TABLE IF NOT EXISTS of_tunnel_services (
    id BIGSERIAL PRIMARY KEY,
    tunnel_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    service_type VARCHAR(16) NOT NULL DEFAULT 'tcp',
    relay_id BIGINT NOT NULL DEFAULT 0,
    listen_port INTEGER NOT NULL DEFAULT 0,
    target_addr VARCHAR(512) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    options_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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
ALTER TABLE of_proxy_routes
    DROP COLUMN IF EXISTS tunnel_options_json,
    DROP COLUMN IF EXISTS tunnel_id,
    ADD COLUMN IF NOT EXISTS tunnel_node_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id BIGINT NOT NULL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NOT NULL,
    frps_connections INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id BIGINT NOT NULL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NOT NULL,
    tunnel_status VARCHAR(16) NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

ALTER TABLE of_nodes
    ADD COLUMN IF NOT EXISTS relay_bind_port INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS relay_vhost_http_port INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS relay_auth_token VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relay_agent_access_addr VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relay_client_access_addr VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relay_client_proxy_url VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relay_status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS relay_web_server_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TRIGGER dirty_edge_relay_insert AFTER INSERT ON of_nodes
FOR EACH ROW WHEN (NEW.node_type = 'tunnel_relay')
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_relay_update AFTER UPDATE OF node_type, ip, relay_vhost_http_port, relay_agent_access_addr, relay_client_access_addr ON of_nodes
FOR EACH ROW WHEN ((NEW.node_type = 'tunnel_relay' OR OLD.node_type = 'tunnel_relay') AND (NEW.node_type IS DISTINCT FROM OLD.node_type OR NEW.ip IS DISTINCT FROM OLD.ip OR NEW.relay_vhost_http_port IS DISTINCT FROM OLD.relay_vhost_http_port OR NEW.relay_agent_access_addr IS DISTINCT FROM OLD.relay_agent_access_addr OR NEW.relay_client_access_addr IS DISTINCT FROM OLD.relay_client_access_addr))
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_relay_delete AFTER DELETE ON of_nodes
FOR EACH ROW WHEN (OLD.node_type = 'tunnel_relay')
EXECUTE FUNCTION openflare_dirty_edge_snapshot();
