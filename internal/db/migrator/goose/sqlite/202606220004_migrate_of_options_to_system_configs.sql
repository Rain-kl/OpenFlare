-- +goose Up
-- 将 of_options 配置迁移到 w_system_configs（仅迁移新配置，已存在的不重复）

-- Agent 相关配置 (business, visibility=0)
INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'agent_discovery_token', COALESCE(value, ''), 'business', 0, 'Agent 发现令牌'
FROM of_options WHERE key = 'AgentDiscoveryToken';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'agent_heartbeat_interval', value, 'business', 0, 'Agent 心跳间隔（毫秒）'
FROM of_options WHERE key = 'AgentHeartbeatInterval';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'agent_websocket_upgrade_enabled', value, 'business', 0, 'Agent WebSocket 升级开关'
FROM of_options WHERE key = 'AgentWebsocketUpgradeEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'node_offline_threshold', value, 'business', 0, '节点离线阈值（毫秒）'
FROM of_options WHERE key = 'NodeOfflineThreshold';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'agent_update_repo', value, 'business', 0, 'Agent 更新仓库'
FROM of_options WHERE key = 'AgentUpdateRepo';

-- 系统功能配置 (business, visibility=0)
INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'geoip_provider', value, 'business', 0, 'GeoIP 服务商'
FROM of_options WHERE key = 'GeoIPProvider';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'database_auto_cleanup_enabled', value, 'business', 0, '数据库自动清理开关'
FROM of_options WHERE key = 'DatabaseAutoCleanupEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'database_auto_cleanup_retention_days', value, 'business', 0, '数据库保留天数'
FROM of_options WHERE key = 'DatabaseAutoCleanupRetentionDays';

-- UptimeKuma 集成配置 (business, visibility=0)
INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_enabled', value, 'business', 0, 'UptimeKuma 集成开关'
FROM of_options WHERE key = 'UptimeKumaEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_url', COALESCE(value, ''), 'business', 0, 'UptimeKuma URL'
FROM of_options WHERE key = 'UptimeKumaUrl';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_username', COALESCE(value, ''), 'business', 0, 'UptimeKuma 用户名'
FROM of_options WHERE key = 'UptimeKumaUsername';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_password', COALESCE(value, ''), 'business', 0, 'UptimeKuma 密码'
FROM of_options WHERE key = 'UptimeKumaPassword';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_monitor_scope', value, 'business', 0, 'UptimeKuma 监控范围'
FROM of_options WHERE key = 'UptimeKumaMonitorScope';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_selected_sites', COALESCE(value, ''), 'business', 0, 'UptimeKuma 选定站点'
FROM of_options WHERE key = 'UptimeKumaSelectedSites';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_sync_interval', value, 'business', 0, 'UptimeKuma 同步间隔（分钟）'
FROM of_options WHERE key = 'UptimeKumaSyncInterval';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_interval', value, 'business', 0, 'UptimeKuma 监控间隔（秒）'
FROM of_options WHERE key = 'UptimeKumaInterval';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_retry', value, 'business', 0, 'UptimeKuma 重试次数'
FROM of_options WHERE key = 'UptimeKumaRetry';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_retry_interval', value, 'business', 0, 'UptimeKuma 重试间隔（秒）'
FROM of_options WHERE key = 'UptimeKumaRetryInterval';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'uptime_kuma_timeout', value, 'business', 0, 'UptimeKuma 超时（秒）'
FROM of_options WHERE key = 'UptimeKumaTimeout';

-- OpenResty 配置（全部 business 类型，visibility=0）
INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_default_server_return_status', value, 'business', 0, '默认服务器返回状态码'
FROM of_options WHERE key = 'OpenRestyDefaultServerReturnStatus';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_worker_processes', value, 'business', 0, 'Worker 进程数'
FROM of_options WHERE key = 'OpenRestyWorkerProcesses';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_worker_connections', value, 'business', 0, 'Worker 连接数'
FROM of_options WHERE key = 'OpenRestyWorkerConnections';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_worker_rlimit_nofile', value, 'business', 0, 'Worker 文件描述符限制'
FROM of_options WHERE key = 'OpenRestyWorkerRlimitNofile';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_events_use', value, 'business', 0, '事件模型'
FROM of_options WHERE key = 'OpenRestyEventsUse';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_events_multi_accept_enabled', value, 'business', 0, '多路接受开关'
FROM of_options WHERE key = 'OpenRestyEventsMultiAcceptEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_keepalive_timeout', value, 'business', 0, 'Keepalive 超时（秒）'
FROM of_options WHERE key = 'OpenRestyKeepaliveTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_keepalive_requests', value, 'business', 0, 'Keepalive 请求数'
FROM of_options WHERE key = 'OpenRestyKeepaliveRequests';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_client_header_timeout', value, 'business', 0, '客户端头超时（秒）'
FROM of_options WHERE key = 'OpenRestyClientHeaderTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_client_body_timeout', value, 'business', 0, '客户端体超时（秒）'
FROM of_options WHERE key = 'OpenRestyClientBodyTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_client_max_body_size', value, 'business', 0, '客户端最大体大小'
FROM of_options WHERE key = 'OpenRestyClientMaxBodySize';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_large_client_header_buffers', value, 'business', 0, '大客户端头缓冲区'
FROM of_options WHERE key = 'OpenRestyLargeClientHeaderBuffers';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_send_timeout', value, 'business', 0, '发送超时（秒）'
FROM of_options WHERE key = 'OpenRestySendTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_resolvers', COALESCE(value, ''), 'business', 0, 'DNS 解析器'
FROM of_options WHERE key = 'OpenRestyResolvers';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_connect_timeout', value, 'business', 0, '代理连接超时（秒）'
FROM of_options WHERE key = 'OpenRestyProxyConnectTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_send_timeout', value, 'business', 0, '代理发送超时（秒）'
FROM of_options WHERE key = 'OpenRestyProxySendTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_read_timeout', value, 'business', 0, '代理读取超时（秒）'
FROM of_options WHERE key = 'OpenRestyProxyReadTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_websocket_enabled', value, 'business', 0, 'WebSocket 支持开关'
FROM of_options WHERE key = 'OpenRestyWebsocketEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_http3_enabled', value, 'business', 0, 'HTTP/3 支持开关'
FROM of_options WHERE key = 'OpenRestyHTTP3Enabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_request_buffering_enabled', value, 'business', 0, '代理请求缓冲开关'
FROM of_options WHERE key = 'OpenRestyProxyRequestBufferingEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_buffering_enabled', value, 'business', 0, '代理响应缓冲开关'
FROM of_options WHERE key = 'OpenRestyProxyBufferingEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_buffers', value, 'business', 0, '代理缓冲区'
FROM of_options WHERE key = 'OpenRestyProxyBuffers';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_buffer_size', value, 'business', 0, '代理缓冲区大小'
FROM of_options WHERE key = 'OpenRestyProxyBufferSize';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_proxy_busy_buffers_size', value, 'business', 0, '代理繁忙缓冲区大小'
FROM of_options WHERE key = 'OpenRestyProxyBusyBuffersSize';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_gzip_enabled', value, 'business', 0, 'Gzip 压缩开关'
FROM of_options WHERE key = 'OpenRestyGzipEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_gzip_min_length', value, 'business', 0, 'Gzip 最小长度'
FROM of_options WHERE key = 'OpenRestyGzipMinLength';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_gzip_comp_level', value, 'business', 0, 'Gzip 压缩级别'
FROM of_options WHERE key = 'OpenRestyGzipCompLevel';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_enabled', value, 'business', 0, '缓存开关'
FROM of_options WHERE key = 'OpenRestyCacheEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_path', COALESCE(value, ''), 'business', 0, '缓存路径'
FROM of_options WHERE key = 'OpenRestyCachePath';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_levels', value, 'business', 0, '缓存层级'
FROM of_options WHERE key = 'OpenRestyCacheLevels';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_inactive', value, 'business', 0, '缓存不活跃时间'
FROM of_options WHERE key = 'OpenRestyCacheInactive';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_max_size', value, 'business', 0, '缓存最大大小'
FROM of_options WHERE key = 'OpenRestyCacheMaxSize';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_key_template', value, 'business', 0, '缓存键模板'
FROM of_options WHERE key = 'OpenRestyCacheKeyTemplate';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_lock_enabled', value, 'business', 0, '缓存锁开关'
FROM of_options WHERE key = 'OpenRestyCacheLockEnabled';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_lock_timeout', value, 'business', 0, '缓存锁超时'
FROM of_options WHERE key = 'OpenRestyCacheLockTimeout';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_cache_use_stale', value, 'business', 0, '缓存失效策略'
FROM of_options WHERE key = 'OpenRestyCacheUseStale';

INSERT OR REPLACE INTO w_system_configs (key, value, type, visibility, description)
SELECT 'openresty_main_config_template', value, 'business', 0, '主配置模板'
FROM of_options WHERE key = 'OpenRestyMainConfigTemplate';

-- +goose Down
-- 回滚时删除所有迁移的配置项
DELETE FROM w_system_configs WHERE key IN (
    'agent_discovery_token',
    'agent_heartbeat_interval',
    'agent_websocket_upgrade_enabled',
    'node_offline_threshold',
    'agent_update_repo',
    'geoip_provider',
    'database_auto_cleanup_enabled',
    'database_auto_cleanup_retention_days',
    'uptime_kuma_enabled',
    'uptime_kuma_url',
    'uptime_kuma_username',
    'uptime_kuma_password',
    'uptime_kuma_monitor_scope',
    'uptime_kuma_selected_sites',
    'uptime_kuma_sync_interval',
    'uptime_kuma_interval',
    'uptime_kuma_retry',
    'uptime_kuma_retry_interval',
    'uptime_kuma_timeout',
    'openresty_default_server_return_status',
    'openresty_worker_processes',
    'openresty_worker_connections',
    'openresty_worker_rlimit_nofile',
    'openresty_events_use',
    'openresty_events_multi_accept_enabled',
    'openresty_keepalive_timeout',
    'openresty_keepalive_requests',
    'openresty_client_header_timeout',
    'openresty_client_body_timeout',
    'openresty_client_max_body_size',
    'openresty_large_client_header_buffers',
    'openresty_send_timeout',
    'openresty_resolvers',
    'openresty_proxy_connect_timeout',
    'openresty_proxy_send_timeout',
    'openresty_proxy_read_timeout',
    'openresty_websocket_enabled',
    'openresty_http3_enabled',
    'openresty_proxy_request_buffering_enabled',
    'openresty_proxy_buffering_enabled',
    'openresty_proxy_buffers',
    'openresty_proxy_buffer_size',
    'openresty_proxy_busy_buffers_size',
    'openresty_gzip_enabled',
    'openresty_gzip_min_length',
    'openresty_gzip_comp_level',
    'openresty_cache_enabled',
    'openresty_cache_path',
    'openresty_cache_levels',
    'openresty_cache_inactive',
    'openresty_cache_max_size',
    'openresty_cache_key_template',
    'openresty_cache_lock_enabled',
    'openresty_cache_lock_timeout',
    'openresty_cache_use_stale',
    'openresty_main_config_template'
);
