-- +goose Up
INSERT INTO of_performance_policies (preset_key, name, settings_json) VALUES
    ('1c1g', '1C1G', '{"threads":1,"upstream_keepalive_pool_size":64,"max_retries":2,"max_h2_streams":8,"max_concurrent_streams":64,"initial_window_size":262144,"initial_connection_window_size":524288,"max_send_buffer_size":131072,"cache_capacity_mib":32,"cache_max_object_mib":4}'),
    ('2c2g', '2C2G', '{"threads":2,"upstream_keepalive_pool_size":128,"max_concurrent_streams":128,"initial_window_size":524288,"cache_capacity_mib":64,"cache_max_object_mib":8}'),
    ('4c8g', '4C8G', '{"threads":4,"upstream_keepalive_pool_size":256,"max_concurrent_streams":256,"max_h2_streams":32,"initial_connection_window_size":4194304,"cache_capacity_mib":256}');

-- +goose Down
UPDATE of_nodes SET performance_policy_id = NULL WHERE performance_policy_id IN (
    SELECT id FROM of_performance_policies WHERE preset_key IN ('1c1g', '2c2g', '4c8g')
);
DELETE FROM of_performance_policies WHERE preset_key IN ('1c1g', '2c2g', '4c8g');
