-- +goose Up
DROP INDEX IF EXISTS idx_of_node_obs_frpc_captured_at;
DROP INDEX IF EXISTS idx_of_node_obs_frpc_node_id;
DROP TABLE IF EXISTS of_node_obs_frpc;

DROP INDEX IF EXISTS idx_of_node_obs_frps_captured_at;
DROP INDEX IF EXISTS idx_of_node_obs_frps_node_id;
DROP TABLE IF EXISTS of_node_obs_frps;

DROP INDEX IF EXISTS idx_of_node_obs_openresty_captured_at;
DROP INDEX IF EXISTS idx_of_node_obs_openresty_node_id;
DROP TABLE IF EXISTS of_node_obs_openresty;

DROP INDEX IF EXISTS idx_of_node_request_reports_window_ended_at;
DROP INDEX IF EXISTS idx_of_node_request_reports_window_started_at;
DROP INDEX IF EXISTS idx_of_node_request_reports_node_id;
DROP TABLE IF EXISTS of_node_request_reports;

DROP INDEX IF EXISTS idx_of_node_metric_snapshots_captured_at;
DROP INDEX IF EXISTS idx_of_node_metric_snapshots_node_id;
DROP TABLE IF EXISTS of_node_metric_snapshots;

-- +goose Down
CREATE TABLE of_node_metric_snapshots (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    cpu_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes BIGINT NOT NULL DEFAULT 0,
    storage_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_read_bytes BIGINT NOT NULL DEFAULT 0,
    disk_write_bytes BIGINT NOT NULL DEFAULT 0,
    network_rx_bytes BIGINT NOT NULL DEFAULT 0,
    network_tx_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_metric_snapshots_node_id ON of_node_metric_snapshots (node_id);
CREATE INDEX idx_of_node_metric_snapshots_captured_at ON of_node_metric_snapshots (captured_at);

CREATE TABLE of_node_request_reports (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    window_ended_at TIMESTAMPTZ NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    unique_visitor_count BIGINT NOT NULL DEFAULT 0,
    status_codes_json TEXT,
    top_domains_json TEXT,
    source_countries_json TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_request_reports_node_id ON of_node_request_reports (node_id);
CREATE INDEX idx_of_node_request_reports_window_started_at ON of_node_request_reports (window_started_at);
CREATE INDEX idx_of_node_request_reports_window_ended_at ON of_node_request_reports (window_ended_at);

CREATE TABLE of_node_obs_openresty (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    openresty_rx_bytes BIGINT NOT NULL DEFAULT 0,
    openresty_tx_bytes BIGINT NOT NULL DEFAULT 0,
    openresty_connections BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_obs_openresty_node_id ON of_node_obs_openresty (node_id);
CREATE INDEX idx_of_node_obs_openresty_captured_at ON of_node_obs_openresty (captured_at);

CREATE TABLE of_node_obs_frps (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    frps_connections INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_obs_frps_node_id ON of_node_obs_frps (node_id);
CREATE INDEX idx_of_node_obs_frps_captured_at ON of_node_obs_frps (captured_at);

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