-- +goose Up
-- Hourly traffic rollups: 30d TTL + UV aggregation semantics.
--
-- unique_visitor_count on of_node_request_reports is per short report window
-- (agent local distinct count for that window only). Summing those values in the
-- MV (and again via SummingMergeTree part merges) invents a "true UV" number that
-- double-counts visitors across windows. Prefer max() as a peak-window estimate;
-- still NOT distinct visitors across the hour — UI/API must not overclaim.

ALTER TABLE of_node_traffic_hourly
    MODIFY TTL toDateTime(hour) + INTERVAL 30 DAY;

DROP VIEW IF EXISTS of_node_traffic_hourly_mv;

CREATE MATERIALIZED VIEW of_node_traffic_hourly_mv
TO of_node_traffic_hourly
AS
SELECT
    node_id,
    toStartOfHour(window_ended_at) AS hour,
    sum(request_count) AS request_count,
    sum(error_count) AS error_count,
    -- Peak per-window UV estimate for the hour; not true cross-window distinct UV.
    max(unique_visitor_count) AS unique_visitor_count
FROM of_node_request_reports
GROUP BY node_id, hour;

-- +goose Down
-- TTL reverse is not safe without table rewrite; restore prior MV definition only.
DROP VIEW IF EXISTS of_node_traffic_hourly_mv;

CREATE MATERIALIZED VIEW of_node_traffic_hourly_mv
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
