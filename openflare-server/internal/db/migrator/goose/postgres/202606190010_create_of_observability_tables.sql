-- +goose Up
CREATE TABLE of_node_system_profiles (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    hostname VARCHAR(255) NOT NULL DEFAULT '',
    os_name VARCHAR(128) NOT NULL DEFAULT '',
    os_version VARCHAR(128) NOT NULL DEFAULT '',
    kernel_version VARCHAR(128) NOT NULL DEFAULT '',
    architecture VARCHAR(64) NOT NULL DEFAULT '',
    cpu_model VARCHAR(255) NOT NULL DEFAULT '',
    cpu_cores INTEGER NOT NULL DEFAULT 0,
    total_memory_bytes BIGINT NOT NULL DEFAULT 0,
    total_disk_bytes BIGINT NOT NULL DEFAULT 0,
    uptime_seconds BIGINT NOT NULL DEFAULT 0,
    reported_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_of_node_system_profiles_node_id ON of_node_system_profiles (node_id);
CREATE INDEX idx_of_node_system_profiles_reported_at ON of_node_system_profiles (reported_at);

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

CREATE TABLE of_node_health_events (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    severity VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL,
    message TEXT,
    first_triggered_at TIMESTAMPTZ NOT NULL,
    last_triggered_at TIMESTAMPTZ NOT NULL,
    reported_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    metadata_json TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_health_events_node_id ON of_node_health_events (node_id);
CREATE INDEX idx_of_node_health_events_event_type ON of_node_health_events (event_type);
CREATE INDEX idx_of_node_health_events_status ON of_node_health_events (status);
CREATE INDEX idx_of_node_health_events_first_triggered_at ON of_node_health_events (first_triggered_at);
CREATE INDEX idx_of_node_health_events_last_triggered_at ON of_node_health_events (last_triggered_at);
CREATE INDEX idx_of_node_health_events_reported_at ON of_node_health_events (reported_at);
CREATE INDEX idx_of_node_health_events_resolved_at ON of_node_health_events (resolved_at);

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

CREATE TABLE of_node_access_logs (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    logged_at TIMESTAMPTZ NOT NULL,
    remote_addr VARCHAR(128) NOT NULL DEFAULT '',
    region VARCHAR(128) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL DEFAULT '',
    path VARCHAR(2048) NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_access_logs_node_id ON of_node_access_logs (node_id);
CREATE INDEX idx_of_node_access_logs_logged_at ON of_node_access_logs (logged_at);
CREATE INDEX idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr);
CREATE INDEX idx_of_node_access_logs_host ON of_node_access_logs (host);
CREATE INDEX idx_of_node_access_logs_status_code ON of_node_access_logs (status_code);

-- +goose Down
DROP TABLE IF EXISTS of_node_access_logs;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_openresty;
DROP TABLE IF EXISTS of_node_health_events;
DROP TABLE IF EXISTS of_node_request_reports;
DROP TABLE IF EXISTS of_node_metric_snapshots;
DROP TABLE IF EXISTS of_node_system_profiles;