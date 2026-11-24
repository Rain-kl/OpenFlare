-- +goose Up
-- Recreate ClickHouse node metric snapshot table and hourly rollup adopting Komari Record schema.
DROP VIEW IF EXISTS of_node_metric_capacity_hourly_mv;
DROP TABLE IF EXISTS of_node_metric_capacity_hourly;
DROP TABLE IF EXISTS of_node_metric_snapshots;

CREATE TABLE IF NOT EXISTS of_node_metric_snapshots
(
    id               UInt64,
    node_id          String,
    captured_at      DateTime64(3, 'UTC'),
    cpu              Float64,
    gpu              Float64,
    ram              Int64,
    ram_total        Int64,
    swap             Int64,
    swap_total       Int64,
    load             Float64,
    load5            Float64,
    load15           Float64,
    temp             Float64,
    disk             Int64,
    disk_total       Int64,
    disk_read_bytes  Int64,
    disk_write_bytes Int64,
    net_in           Int64,
    net_out          Int64,
    net_total_up     Int64,
    net_total_down   Int64,
    process          Int32,
    connections      Int32,
    connections_udp  Int32,
    uptime           Int64,
    created_at       DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY;

CREATE TABLE IF NOT EXISTS of_node_metric_capacity_hourly
(
    node_id            String,
    hour               DateTime,
    cpu_usage_sum      SimpleAggregateFunction(sum, Float64),
    cpu_usage_count    SimpleAggregateFunction(sum, UInt64),
    memory_usage_sum   SimpleAggregateFunction(sum, Float64),
    memory_usage_count SimpleAggregateFunction(sum, UInt64),
    network_rx_min     SimpleAggregateFunction(min, Int64),
    network_rx_max     SimpleAggregateFunction(max, Int64),
    network_tx_min     SimpleAggregateFunction(min, Int64),
    network_tx_max     SimpleAggregateFunction(max, Int64),
    disk_read_min      SimpleAggregateFunction(min, Int64),
    disk_read_max      SimpleAggregateFunction(max, Int64),
    disk_write_min     SimpleAggregateFunction(min, Int64),
    disk_write_max     SimpleAggregateFunction(max, Int64)
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
    sum(cpu) AS cpu_usage_sum,
    toUInt64(count()) AS cpu_usage_count,
    sum(if(ram_total > 0, (ram * 100.0) / ram_total, 0)) AS memory_usage_sum,
    toUInt64(countIf(ram_total > 0)) AS memory_usage_count,
    min(net_total_down) AS network_rx_min,
    max(net_total_down) AS network_rx_max,
    min(net_total_up) AS network_tx_min,
    max(net_total_up) AS network_tx_max,
    min(disk_read_bytes) AS disk_read_min,
    max(disk_read_bytes) AS disk_read_max,
    min(disk_write_bytes) AS disk_write_min,
    max(disk_write_bytes) AS disk_write_max
FROM of_node_metric_snapshots
GROUP BY node_id, hour;

CREATE TABLE IF NOT EXISTS of_node_ping_records
(
    id          UInt64,
    node_id     String,
    task_id     UInt64,
    recorded_at DateTime64(3, 'UTC'),
    value       Int32,
    created_at  DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (node_id, task_id, recorded_at, id)
TTL toDateTime(recorded_at) + INTERVAL 30 DAY;

-- +goose Down
DROP TABLE IF EXISTS of_node_ping_records;
DROP VIEW IF EXISTS of_node_metric_capacity_hourly_mv;
DROP TABLE IF EXISTS of_node_metric_capacity_hourly;
DROP TABLE IF EXISTS of_node_metric_snapshots;
