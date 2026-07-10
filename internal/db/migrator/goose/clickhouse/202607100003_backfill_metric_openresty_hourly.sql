-- +goose Up
-- One-time historical backfill for hours not yet present in rollup tables.
-- MV only ingests rows after creation; without this, 24h charts rely on raw merge forever.
-- ANTI JOIN avoids double-counting hours already filled by the live MV.

INSERT INTO of_node_metric_capacity_hourly
SELECT
    s.node_id,
    toStartOfHour(s.captured_at) AS hour,
    sum(s.cpu_usage_percent) AS cpu_usage_sum,
    toUInt64(count()) AS cpu_usage_count,
    sum(if(s.memory_total_bytes > 0, (s.memory_used_bytes * 100.0) / s.memory_total_bytes, 0)) AS memory_usage_sum,
    toUInt64(countIf(s.memory_total_bytes > 0)) AS memory_usage_count,
    min(s.network_rx_bytes) AS network_rx_min,
    max(s.network_rx_bytes) AS network_rx_max,
    min(s.network_tx_bytes) AS network_tx_min,
    max(s.network_tx_bytes) AS network_tx_max,
    min(s.disk_read_bytes) AS disk_read_min,
    max(s.disk_read_bytes) AS disk_read_max,
    min(s.disk_write_bytes) AS disk_write_min,
    max(s.disk_write_bytes) AS disk_write_max
FROM of_node_metric_snapshots AS s
ANTI JOIN
(
    SELECT
        node_id,
        hour
    FROM of_node_metric_capacity_hourly
    GROUP BY
        node_id,
        hour
) AS existing
ON s.node_id = existing.node_id AND toStartOfHour(s.captured_at) = existing.hour
WHERE s.captured_at >= now() - INTERVAL 30 DAY
GROUP BY
    s.node_id,
    hour;

INSERT INTO of_node_openresty_hourly
SELECT
    s.node_id,
    toStartOfHour(s.captured_at) AS hour,
    min(s.openresty_rx_bytes) AS openresty_rx_min,
    max(s.openresty_rx_bytes) AS openresty_rx_max,
    min(s.openresty_tx_bytes) AS openresty_tx_min,
    max(s.openresty_tx_bytes) AS openresty_tx_max
FROM of_node_obs_openresty AS s
ANTI JOIN
(
    SELECT
        node_id,
        hour
    FROM of_node_openresty_hourly
    GROUP BY
        node_id,
        hour
) AS existing
ON s.node_id = existing.node_id AND toStartOfHour(s.captured_at) = existing.hour
WHERE s.captured_at >= now() - INTERVAL 30 DAY
GROUP BY
    s.node_id,
    hour;

-- +goose Down
-- Backfill is additive; down does not remove historical rollup rows (TTL still applies).
SELECT 1;
