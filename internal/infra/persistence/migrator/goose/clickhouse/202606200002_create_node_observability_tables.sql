-- +goose Up
CREATE TABLE IF NOT EXISTS of_node_metric_snapshots
(
    id                  UInt64,
    node_id             String,
    captured_at         DateTime64(3, 'UTC'),
    cpu_usage_percent   Float64,
    memory_used_bytes   Int64,
    memory_total_bytes  Int64,
    storage_used_bytes  Int64,
    storage_total_bytes Int64,
    disk_read_bytes     Int64,
    disk_write_bytes    Int64,
    network_rx_bytes    Int64,
    network_tx_bytes    Int64,
    created_at          DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS of_node_request_reports
(
    id                    UInt64,
    node_id               String,
    window_started_at     DateTime64(3, 'UTC'),
    window_ended_at       DateTime64(3, 'UTC'),
    request_count         Int64,
    error_count           Int64,
    unique_visitor_count  Int64,
    status_codes_json     String,
    top_domains_json      String,
    source_countries_json String,
    created_at            DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(window_ended_at)
ORDER BY (node_id, window_ended_at, window_started_at, id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS of_node_obs_openresty
(
    id                    UInt64,
    node_id               String,
    captured_at           DateTime64(3, 'UTC'),
    openresty_rx_bytes    Int64,
    openresty_tx_bytes    Int64,
    openresty_connections Int64,
    created_at            DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS of_node_obs_frps
(
    id                 UInt64,
    node_id            String,
    captured_at        DateTime64(3, 'UTC'),
    frps_connections   Int32,
    frps_proxy_count   Int32,
    frps_client_count  Int32,
    frps_proxies       String,
    created_at         DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS of_node_obs_frpc
(
    id                      UInt64,
    node_id                 String,
    captured_at             DateTime64(3, 'UTC'),
    tunnel_status           String,
    connected_relays_count  Int32,
    created_at              DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
SETTINGS index_granularity = 8192;

-- +goose Down
DROP TABLE IF EXISTS of_node_obs_frpc;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_openresty;
DROP TABLE IF EXISTS of_node_request_reports;
DROP TABLE IF EXISTS of_node_metric_snapshots;