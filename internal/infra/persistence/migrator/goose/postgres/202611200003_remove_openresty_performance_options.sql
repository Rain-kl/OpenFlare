-- +goose Up
DELETE FROM w_system_configs WHERE key IN (
    'pingora_enabled',
    'openresty_cache_enabled',
    'openresty_cache_inactive',
    'openresty_cache_key_template',
    'openresty_cache_levels',
    'openresty_cache_lock_enabled',
    'openresty_cache_lock_timeout',
    'openresty_cache_max_size',
    'openresty_cache_path',
    'openresty_cache_use_stale',
    'openresty_client_body_timeout',
    'openresty_client_header_timeout',
    'openresty_client_max_body_size',
    'openresty_default_server_return_status',
    'openresty_events_multi_accept_enabled',
    'openresty_events_use',
    'openresty_gzip_comp_level',
    'openresty_gzip_enabled',
    'openresty_gzip_min_length',
    'openresty_http3_enabled',
    'openresty_keepalive_requests',
    'openresty_keepalive_timeout',
    'openresty_large_client_header_buffers',
    'openresty_main_config_template',
    'openresty_proxy_buffer_size',
    'openresty_proxy_buffering_enabled',
    'openresty_proxy_buffers',
    'openresty_proxy_busy_buffers_size',
    'openresty_proxy_connect_timeout',
    'openresty_proxy_read_timeout',
    'openresty_proxy_request_buffering_enabled',
    'openresty_proxy_send_timeout',
    'openresty_resolvers',
    'openresty_send_timeout',
    'openresty_websocket_enabled',
    'openresty_worker_connections',
    'openresty_worker_processes',
    'openresty_worker_rlimit_nofile'
);

-- Preserve shared security policy under engine-independent keys.
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
SELECT 'edge_' || substr(key, 11), value, type, visibility, description, created_at, updated_at
FROM w_system_configs WHERE key IN ('openresty_trusted_proxy_cidrs', 'openresty_default_limit_conn_per_server', 'openresty_default_limit_conn_per_ip', 'openresty_default_limit_rate', 'openresty_default_limit_req_per_ip')
ON CONFLICT (key) DO NOTHING;
DELETE FROM w_system_configs WHERE key IN ('openresty_trusted_proxy_cidrs', 'openresty_default_limit_conn_per_server', 'openresty_default_limit_conn_per_ip', 'openresty_default_limit_rate', 'openresty_default_limit_req_per_ip');

UPDATE of_config_versions SET is_active = false
WHERE engine = 'openresty' AND EXISTS (
    SELECT 1 FROM of_config_versions WHERE engine = 'pingora' AND is_active = true
);

-- +goose Down
SELECT 1;
