-- +goose Up
-- M5: L2 edge health table, L1 access-log hourly rollup, drop deprecated pre-aggregation tables.

CREATE TABLE IF NOT EXISTS of_node_edge_health
(
    id           UInt64,
    node_id      String,
    captured_at  DateTime64(3, 'UTC'),
    status       LowCardinality(String),
    connections  Int64,
    created_at   DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Hourly business traffic from access logs (Server-side only; Agent never writes this).
-- SummingMergeTree merges request/error/bytes; UV is not stored (query raw for exact UV).
CREATE TABLE IF NOT EXISTS of_access_log_hourly
(
    node_id         String,
    hour            DateTime('UTC'),
    host            String,
    request_count   UInt64,
    error_count     UInt64,
    bytes_sent      UInt64,
    request_length  UInt64
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (node_id, hour, host)
TTL hour + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS of_access_log_hourly_mv
TO of_access_log_hourly
AS
SELECT
    node_id,
    toStartOfHour(logged_at) AS hour,
    host,
    toUInt64(count()) AS request_count,
    toUInt64(countIf(status_code >= 500)) AS error_count,
    sum(bytes_sent) AS bytes_sent,
    sum(request_length) AS request_length
FROM of_node_access_logs
GROUP BY node_id, hour, host;

-- Deprecated pre-aggregation paths (business traffic is access logs; OR throughput is not authoritative).
DROP VIEW IF EXISTS of_node_traffic_hourly_mv;
DROP TABLE IF EXISTS of_node_traffic_hourly;
DROP VIEW IF EXISTS of_node_openresty_hourly_mv;
DROP TABLE IF EXISTS of_node_openresty_hourly;
DROP TABLE IF EXISTS of_node_request_reports;
DROP TABLE IF EXISTS of_node_obs_openresty;

-- +goose Down
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
TTL toDateTime(captured_at) + INTERVAL 30 DAY
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
TTL toDateTime(window_ended_at) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS of_node_traffic_hourly
(
    node_id               String,
    hour                  DateTime,
    request_count         UInt64,
    error_count           UInt64,
    unique_visitor_count  UInt64
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (node_id, hour);

CREATE MATERIALIZED VIEW IF NOT EXISTS of_node_traffic_hourly_mv
TO of_node_traffic_hourly
AS
SELECT
    node_id,
    toStartOfHour(window_ended_at) AS hour,
    sum(request_count) AS request_count,
    sum(error_count) AS error_count,
    max(unique_visitor_count) AS unique_visitor_count
FROM of_node_request_reports
GROUP BY node_id, hour;

CREATE TABLE IF NOT EXISTS of_node_openresty_hourly
(
    node_id            String,
    hour               DateTime,
    openresty_rx_min   SimpleAggregateFunction(min, Int64),
    openresty_rx_max   SimpleAggregateFunction(max, Int64),
    openresty_tx_min   SimpleAggregateFunction(min, Int64),
    openresty_tx_max   SimpleAggregateFunction(max, Int64)
)
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (node_id, hour)
TTL hour + INTERVAL 30 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS of_node_openresty_hourly_mv
TO of_node_openresty_hourly
AS
SELECT
    node_id,
    toStartOfHour(captured_at) AS hour,
    min(openresty_rx_bytes) AS openresty_rx_min,
    max(openresty_rx_bytes) AS openresty_rx_max,
    min(openresty_tx_bytes) AS openresty_tx_min,
    max(openresty_tx_bytes) AS openresty_tx_max
FROM of_node_obs_openresty
GROUP BY node_id, hour;

DROP VIEW IF EXISTS of_access_log_hourly_mv;
DROP TABLE IF EXISTS of_access_log_hourly;
DROP TABLE IF EXISTS of_node_edge_health;
