-- +goose Up
-- Hourly capacity rollups (avg CPU/memory + counter min/max for in-hour delta approximation).
-- Network/disk counters are cumulative; max-min within an hour approximates that hour's delta
-- (cross-hour continuity is intentionally approximate for dashboard trends).
CREATE TABLE IF NOT EXISTS of_node_metric_capacity_hourly
(
    node_id              String,
    hour                 DateTime,
    cpu_usage_sum        SimpleAggregateFunction(sum, Float64),
    cpu_usage_count      SimpleAggregateFunction(sum, UInt64),
    memory_usage_sum     SimpleAggregateFunction(sum, Float64),
    memory_usage_count   SimpleAggregateFunction(sum, UInt64),
    network_rx_min       SimpleAggregateFunction(min, Int64),
    network_rx_max       SimpleAggregateFunction(max, Int64),
    network_tx_min       SimpleAggregateFunction(min, Int64),
    network_tx_max       SimpleAggregateFunction(max, Int64),
    disk_read_min        SimpleAggregateFunction(min, Int64),
    disk_read_max        SimpleAggregateFunction(max, Int64),
    disk_write_min       SimpleAggregateFunction(min, Int64),
    disk_write_max       SimpleAggregateFunction(max, Int64)
)
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (node_id, hour)
TTL hour + INTERVAL 30 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS of_node_metric_capacity_hourly_mv
TO of_node_metric_capacity_hourly
AS
SELECT
    node_id,
    toStartOfHour(captured_at) AS hour,
    sum(cpu_usage_percent) AS cpu_usage_sum,
    toUInt64(count()) AS cpu_usage_count,
    sum(if(memory_total_bytes > 0, (memory_used_bytes * 100.0) / memory_total_bytes, 0)) AS memory_usage_sum,
    toUInt64(countIf(memory_total_bytes > 0)) AS memory_usage_count,
    min(network_rx_bytes) AS network_rx_min,
    max(network_rx_bytes) AS network_rx_max,
    min(network_tx_bytes) AS network_tx_min,
    max(network_tx_bytes) AS network_tx_max,
    min(disk_read_bytes) AS disk_read_min,
    max(disk_read_bytes) AS disk_read_max,
    min(disk_write_bytes) AS disk_write_min,
    max(disk_write_bytes) AS disk_write_max
FROM of_node_metric_snapshots
GROUP BY node_id, hour;

-- Hourly OpenResty counter rollups (min/max per node-hour for delta approximation).
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

-- +goose Down
DROP VIEW IF EXISTS of_node_openresty_hourly_mv;
DROP TABLE IF EXISTS of_node_openresty_hourly;
DROP VIEW IF EXISTS of_node_metric_capacity_hourly_mv;
DROP TABLE IF EXISTS of_node_metric_capacity_hourly;
