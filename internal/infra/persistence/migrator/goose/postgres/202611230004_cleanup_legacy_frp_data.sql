-- +goose Up
UPDATE of_proxy_routes
SET enabled = FALSE,
    tunnel_node_id = NULL,
    tunnel_target_addr = '',
    tunnel_target_protocol = ''
WHERE upstream_type = 'tunnel';

DELETE FROM of_node_system_profiles
WHERE node_id IN (SELECT node_id FROM of_nodes WHERE node_type IN ('tunnel_relay', 'tunnel_client'));

DELETE FROM of_node_health_events
WHERE node_id IN (SELECT node_id FROM of_nodes WHERE node_type IN ('tunnel_relay', 'tunnel_client'));

DELETE FROM of_apply_logs
WHERE node_id IN (SELECT node_id FROM of_nodes WHERE node_type IN ('tunnel_relay', 'tunnel_client'));

DELETE FROM of_node_cert_deliveries
WHERE node_id IN (SELECT node_id FROM of_nodes WHERE node_type IN ('tunnel_relay', 'tunnel_client'));

DELETE FROM of_node_metric_snapshots
WHERE node_id IN (SELECT node_id FROM of_nodes WHERE node_type IN ('tunnel_relay', 'tunnel_client'));

DELETE FROM of_nodes
WHERE node_type IN ('tunnel_relay', 'tunnel_client');

DELETE FROM w_system_configs
WHERE key IN ('relay_frps_web_ui_enabled', 'relay_frps_web_ui_port');

-- +goose Down
SELECT 1;
