-- +goose Up
-- One-time historical backfill for of_access_log_hourly.
-- MV only ingests rows after creation; without this, 24h charts fall back to raw access logs.
-- ANTI JOIN avoids double-counting (node_id, hour, host) already filled by the live MV.
-- UV is intentionally NOT stored in of_access_log_hourly (SummingMergeTree counts only).

INSERT INTO of_access_log_hourly
SELECT
    s.node_id,
    toStartOfHour(s.logged_at) AS hour,
    s.host,
    toUInt64(count()) AS request_count,
    toUInt64(countIf(s.status_code >= 500)) AS error_count,
    sum(s.bytes_sent) AS bytes_sent,
    sum(s.request_length) AS request_length
FROM of_node_access_logs AS s
ANTI JOIN
(
    SELECT
        node_id,
        hour,
        host
    FROM of_access_log_hourly
    GROUP BY
        node_id,
        hour,
        host
) AS existing
ON s.node_id = existing.node_id
    AND toStartOfHour(s.logged_at) = existing.hour
    AND s.host = existing.host
WHERE s.logged_at >= now() - INTERVAL 90 DAY
GROUP BY
    s.node_id,
    hour,
    s.host;

-- +goose Down
-- Backfill is additive; down does not remove historical rollup rows (TTL still applies).
SELECT 1;
