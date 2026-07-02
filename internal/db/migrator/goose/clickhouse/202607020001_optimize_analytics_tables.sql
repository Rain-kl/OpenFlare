-- +goose Up
-- Add TTL policies to analytics tables so ClickHouse can expire rows automatically.
ALTER TABLE w_user_access_logs MODIFY TTL created_at + INTERVAL 180 DAY;

ALTER TABLE of_node_access_logs MODIFY TTL logged_at + INTERVAL 90 DAY;

ALTER TABLE of_node_metric_snapshots MODIFY TTL captured_at + INTERVAL 30 DAY;

ALTER TABLE of_node_request_reports MODIFY TTL window_ended_at + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_openresty MODIFY TTL captured_at + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_frps MODIFY TTL captured_at + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_frpc MODIFY TTL captured_at + INTERVAL 30 DAY;

-- Narrow ORDER BY for node access logs to match common filter patterns.
-- Requires ClickHouse 24.10+ (MODIFY ORDER BY). On older versions this statement
-- may fail and require manual table recreation; TTL changes above are still safe.
ALTER TABLE of_node_access_logs MODIFY ORDER BY (node_id, logged_at, status_code);

-- +goose Down
-- TTL and ORDER BY changes cannot be safely reversed without recreating tables.