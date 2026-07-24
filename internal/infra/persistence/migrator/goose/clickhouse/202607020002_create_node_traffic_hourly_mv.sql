-- +goose Up
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
    sum(unique_visitor_count) AS unique_visitor_count
FROM of_node_request_reports
GROUP BY node_id, hour;

-- +goose Down
DROP VIEW IF EXISTS of_node_traffic_hourly_mv;
DROP TABLE IF EXISTS of_node_traffic_hourly;