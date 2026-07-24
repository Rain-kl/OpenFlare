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
CREATE TABLE IF NOT EXISTS of_node_metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    cpu_usage_percent REAL NOT NULL DEFAULT 0,
    memory_used_bytes INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes INTEGER NOT NULL DEFAULT 0,
    storage_used_bytes INTEGER NOT NULL DEFAULT 0,
    storage_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes INTEGER NOT NULL DEFAULT 0,
    network_rx_bytes INTEGER NOT NULL DEFAULT 0,
    network_tx_bytes INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node_id ON of_node_metric_snapshots (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_captured_at ON of_node_metric_snapshots (captured_at);

CREATE TABLE IF NOT EXISTS of_node_request_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    window_started_at DATETIME NOT NULL,
    window_ended_at DATETIME NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    unique_visitor_count INTEGER NOT NULL DEFAULT 0,
    status_codes_json TEXT,
    top_domains_json TEXT,
    source_countries_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_node_request_reports_node_id ON of_node_request_reports (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_request_reports_window_started_at ON of_node_request_reports (window_started_at);
CREATE INDEX IF NOT EXISTS idx_of_node_request_reports_window_ended_at ON of_node_request_reports (window_ended_at);

CREATE TABLE IF NOT EXISTS of_node_obs_openresty (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    openresty_rx_bytes INTEGER NOT NULL DEFAULT 0,
    openresty_tx_bytes INTEGER NOT NULL DEFAULT 0,
    openresty_connections INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_node_obs_openresty_node_id ON of_node_obs_openresty (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_openresty_captured_at ON of_node_obs_openresty (captured_at);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    frps_connections INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node_id ON of_node_obs_frps (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_captured_at ON of_node_obs_frps (captured_at);

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