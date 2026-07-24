-- +goose Up
-- Add TTL policies to analytics tables so ClickHouse can expire rows automatically.
ALTER TABLE w_user_access_logs MODIFY TTL created_at + INTERVAL 180 DAY;

-- DateTime64 columns must be cast for TTL (ClickHouse requires DateTime/Date in TTL expr).
ALTER TABLE of_node_access_logs MODIFY TTL toDateTime(logged_at) + INTERVAL 90 DAY;

ALTER TABLE of_node_metric_snapshots MODIFY TTL toDateTime(captured_at) + INTERVAL 30 DAY;

ALTER TABLE of_node_request_reports MODIFY TTL toDateTime(window_ended_at) + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_openresty MODIFY TTL toDateTime(captured_at) + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_frps MODIFY TTL toDateTime(captured_at) + INTERVAL 30 DAY;

ALTER TABLE of_node_obs_frpc MODIFY TTL toDateTime(captured_at) + INTERVAL 30 DAY;

-- +goose Down
-- TTL changes cannot be safely reversed without recreating tables.