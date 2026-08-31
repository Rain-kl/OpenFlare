-- +goose Up
CREATE TABLE IF NOT EXISTS of_acme_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    private_key TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS of_apply_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    version TEXT NOT NULL,
    result TEXT NOT NULL,
    message TEXT,
    checksum TEXT NOT NULL DEFAULT '',
    main_config_checksum TEXT NOT NULL DEFAULT '',
    route_config_checksum TEXT NOT NULL DEFAULT '',
    support_file_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_of_apply_logs_created_at ON of_apply_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_of_apply_logs_node_id ON of_apply_logs(node_id);

CREATE TABLE IF NOT EXISTS of_cf_connections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL DEFAULT '',
    dns_account_id INTEGER,
    authorization TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    verified_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_connections_dns_account_id ON of_cf_connections (dns_account_id);

CREATE TABLE IF NOT EXISTS of_cf_pointing_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    primary_node_id INTEGER NOT NULL,
    backup_node_id INTEGER,
    active_node_id INTEGER NOT NULL,
    default_proxied BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_active_node_id ON of_cf_pointing_groups (active_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_backup_node_id ON of_cf_pointing_groups (backup_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_primary_node_id ON of_cf_pointing_groups (primary_node_id);

CREATE TABLE IF NOT EXISTS of_cf_pointing_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    zone_domain_id INTEGER NOT NULL,
    proxied BOOLEAN NOT NULL DEFAULT FALSE,
    cf_zone_id TEXT NOT NULL DEFAULT '',
    cf_record_id TEXT NOT NULL DEFAULT '',
    desired_ip TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT '',
    synced_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_members_group_id ON of_cf_pointing_members (group_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_cf_pointing_members_zone_domain_id ON of_cf_pointing_members (zone_domain_id);

CREATE TABLE IF NOT EXISTS of_config_versions (
    version VARCHAR(32) PRIMARY KEY,
    snapshot_json TEXT NOT NULL,
    main_config TEXT NOT NULL DEFAULT '',
    rendered_config TEXT NOT NULL,
    support_files_json TEXT NOT NULL DEFAULT '[]',
    checksum VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_config_versions_is_active ON of_config_versions (is_active);

CREATE TABLE IF NOT EXISTS of_dns_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    authorization TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS of_node_access_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id         TEXT NOT NULL DEFAULT '',
    logged_at       DATETIME NOT NULL,
    remote_addr     TEXT NOT NULL DEFAULT '',
    region          TEXT NOT NULL DEFAULT '',
    host            TEXT NOT NULL DEFAULT '',
    path            TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    TEXT NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      INTEGER NOT NULL DEFAULT 0,
    request_length  INTEGER NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host ON of_node_access_logs (host, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host_lower ON of_node_access_logs (lower(trim(host)));
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_logged_at ON of_node_access_logs (logged_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_node_id ON of_node_access_logs (node_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_status_code ON of_node_access_logs (status_code, logged_at DESC);

CREATE TABLE IF NOT EXISTS of_node_edge_health (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id     TEXT NOT NULL DEFAULT '',
    captured_at DATETIME NOT NULL,
    status      TEXT NOT NULL DEFAULT '',
    connections INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_health_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    first_triggered_at DATETIME NOT NULL,
    last_triggered_at DATETIME NOT NULL,
    reported_at DATETIME NOT NULL,
    resolved_at DATETIME,
    metadata_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_event_type ON of_node_health_events (event_type);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_first_triggered_at ON of_node_health_events (first_triggered_at);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_last_triggered_at ON of_node_health_events (last_triggered_at);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_node_id ON of_node_health_events (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_reported_at ON of_node_health_events (reported_at);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_resolved_at ON of_node_health_events (resolved_at);
CREATE INDEX IF NOT EXISTS idx_of_node_health_events_status ON of_node_health_events (status);

CREATE TABLE IF NOT EXISTS of_node_metric_snapshots (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id             TEXT NOT NULL DEFAULT '',
    captured_at         DATETIME NOT NULL,
    cpu_usage_percent   REAL NOT NULL DEFAULT 0,
    memory_used_bytes   INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes  INTEGER NOT NULL DEFAULT 0,
    storage_used_bytes  INTEGER NOT NULL DEFAULT 0,
    storage_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes     INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes    INTEGER NOT NULL DEFAULT 0,
    network_rx_bytes    INTEGER NOT NULL DEFAULT 0,
    network_tx_bytes    INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node ON of_node_metric_snapshots (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id                TEXT NOT NULL DEFAULT '',
    captured_at            DATETIME NOT NULL,
    tunnel_status          TEXT NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id           TEXT NOT NULL DEFAULT '',
    captured_at       DATETIME NOT NULL,
    frps_connections  INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count  INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies      TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_system_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    os_name TEXT NOT NULL DEFAULT '',
    os_version TEXT NOT NULL DEFAULT '',
    kernel_version TEXT NOT NULL DEFAULT '',
    architecture TEXT NOT NULL DEFAULT '',
    cpu_model TEXT NOT NULL DEFAULT '',
    cpu_cores INTEGER NOT NULL DEFAULT 0,
    total_memory_bytes INTEGER NOT NULL DEFAULT 0,
    total_disk_bytes INTEGER NOT NULL DEFAULT 0,
    uptime_seconds INTEGER NOT NULL DEFAULT 0,
    reported_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_node_system_profiles_node_id ON of_node_system_profiles (node_id);
CREATE INDEX IF NOT EXISTS idx_of_node_system_profiles_reported_at ON of_node_system_profiles (reported_at);

CREATE TABLE IF NOT EXISTS of_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ip TEXT NOT NULL DEFAULT '',
    ip_manual_override INTEGER NOT NULL DEFAULT 0,
    geo_name TEXT NOT NULL DEFAULT '',
    geo_latitude REAL,
    geo_longitude REAL,
    geo_manual_override INTEGER NOT NULL DEFAULT 0,
    access_token TEXT NOT NULL DEFAULT '',
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    update_requested INTEGER NOT NULL DEFAULT 0,
    update_channel TEXT NOT NULL DEFAULT 'stable',
    update_tag TEXT NOT NULL DEFAULT '',
    restart_openresty_requested INTEGER NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT '',
    ext_version TEXT NOT NULL DEFAULT '',
    openresty_status TEXT NOT NULL DEFAULT 'unknown',
    openresty_message TEXT,
    status TEXT NOT NULL DEFAULT 'offline',
    current_version TEXT NOT NULL DEFAULT '',
    last_seen_at DATETIME,
    last_error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    node_type TEXT NOT NULL DEFAULT 'edge_node',
    relay_bind_port INTEGER NOT NULL DEFAULT 0,
    relay_vhost_http_port INTEGER NOT NULL DEFAULT 0,
    relay_auth_token TEXT NOT NULL DEFAULT '',
    relay_agent_access_addr TEXT NOT NULL DEFAULT '',
    relay_client_access_addr TEXT NOT NULL DEFAULT '',
    relay_client_proxy_url TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    relay_status TEXT NOT NULL DEFAULT 'unknown',
    relay_web_server_enabled INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_of_nodes_access_token ON of_nodes (access_token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_nodes_node_id ON of_nodes (node_id);

CREATE TABLE IF NOT EXISTS of_origins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_origins_address ON of_origins (address);

CREATE TABLE IF NOT EXISTS of_pages_deployment_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    checksum TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployment_files_deployment_id ON of_pages_deployment_files (deployment_id);

CREATE TABLE IF NOT EXISTS of_pages_deployments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    deployment_number INTEGER NOT NULL,
    checksum TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploaded',
    artifact_path TEXT NOT NULL DEFAULT '',
    file_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    activated_at DATETIME,
    upload_id INTEGER NOT NULL DEFAULT 0,
    source_type TEXT NOT NULL DEFAULT '',
    source_identity TEXT,
    source_revision TEXT,
    source_label TEXT NOT NULL DEFAULT '',
    source_meta TEXT NOT NULL DEFAULT '',
    trigger_type TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_checksum ON of_pages_deployments (checksum);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_project_id ON of_pages_deployments (project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_deployments_project_number
    ON of_pages_deployments (project_id, deployment_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_deployments_source_revision
    ON of_pages_deployments (project_id, source_identity, source_revision)
    WHERE source_identity IS NOT NULL AND source_revision IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_status ON of_pages_deployments (status);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_upload_id ON of_pages_deployments (upload_id);

CREATE TABLE IF NOT EXISTS of_pages_project_source_runtime (
    source_id INTEGER PRIMARY KEY,
    etag TEXT NOT NULL DEFAULT '',
    last_seen_revision TEXT NOT NULL DEFAULT '',
    last_seen_detail TEXT NOT NULL DEFAULT '',
    last_applied_revision TEXT NOT NULL DEFAULT '',
    last_applied_detail TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at DATETIME,
    last_synced_at DATETIME,
    next_check_at DATETIME,
    lease_expires_at DATETIME,
    lease_token TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_pages_project_source_runtime_next_check_at
    ON of_pages_project_source_runtime (next_check_at);

CREATE TABLE IF NOT EXISTS of_pages_project_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    remote_url TEXT NOT NULL DEFAULT '',
    allow_insecure INTEGER NOT NULL DEFAULT 0,
    github_repository TEXT NOT NULL DEFAULT '',
    release_selector TEXT NOT NULL DEFAULT '',
    release_tag TEXT NOT NULL DEFAULT '',
    asset_name TEXT NOT NULL DEFAULT '',
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    check_interval_minutes INTEGER NOT NULL DEFAULT 0,
    config_version INTEGER NOT NULL DEFAULT 0,
    source_identity TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_project_sources_project_id
    ON of_pages_project_sources (project_id);

CREATE TABLE IF NOT EXISTS of_pages_projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    spa_fallback_enabled INTEGER NOT NULL DEFAULT 0,
    spa_fallback_path TEXT NOT NULL DEFAULT '/index.html',
    api_proxy_enabled INTEGER NOT NULL DEFAULT 0,
    api_proxy_path TEXT NOT NULL DEFAULT '',
    api_proxy_pass TEXT NOT NULL DEFAULT '',
    api_proxy_rewrite TEXT NOT NULL DEFAULT '',
    active_deployment_id INTEGER,
    root_dir TEXT NOT NULL DEFAULT '',
    entry_file TEXT NOT NULL DEFAULT 'index.html',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    content_config_version INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_of_pages_projects_active_deployment_id ON of_pages_projects (active_deployment_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_projects_slug ON of_pages_projects (slug);

CREATE TABLE IF NOT EXISTS of_proxy_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_name TEXT NOT NULL DEFAULT '',
    origin_id INTEGER,
    origin_url TEXT NOT NULL,
    origin_host TEXT NOT NULL DEFAULT '',
    upstreams TEXT NOT NULL DEFAULT '[]',
    enabled INTEGER NOT NULL DEFAULT 1,
    enable_https INTEGER NOT NULL DEFAULT 0,
    redirect_http INTEGER NOT NULL DEFAULT 0,
    limit_conn_per_server INTEGER NOT NULL DEFAULT 0,
    limit_conn_per_ip INTEGER NOT NULL DEFAULT 0,
    limit_rate TEXT NOT NULL DEFAULT '',
    cache_enabled INTEGER NOT NULL DEFAULT 0,
    cache_policy TEXT NOT NULL DEFAULT '',
    cache_rules TEXT NOT NULL DEFAULT '[]',
    custom_headers TEXT NOT NULL DEFAULT '[]',
    basic_auth_enabled INTEGER NOT NULL DEFAULT 0,
    basic_auth_username TEXT NOT NULL DEFAULT '',
    basic_auth_password TEXT NOT NULL DEFAULT '',
    upstream_type TEXT NOT NULL DEFAULT 'direct',
    tunnel_node_id INTEGER,
    tunnel_target_addr TEXT NOT NULL DEFAULT '',
    tunnel_target_protocol TEXT NOT NULL DEFAULT '',
    pages_project_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    limit_req_per_ip VARCHAR(32) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_origin_id ON of_proxy_routes (origin_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_pages_project_id ON of_proxy_routes (pages_project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_site_name ON of_proxy_routes (site_name);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);

CREATE TABLE IF NOT EXISTS of_tls_certificates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    cert_pem TEXT NOT NULL,
    key_pem TEXT NOT NULL,
    not_before DATETIME,
    not_after DATETIME,
    remark TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT 'upload',
    acme_account_id INTEGER NOT NULL DEFAULT 0,
    dns_account_id INTEGER NOT NULL DEFAULT 0,
    key_algorithm TEXT NOT NULL DEFAULT '',
    auto_renew INTEGER NOT NULL DEFAULT 0,
    primary_domain TEXT NOT NULL DEFAULT '',
    other_domains TEXT NOT NULL DEFAULT '',
    disable_cname INTEGER NOT NULL DEFAULT 0,
    skip_dns INTEGER NOT NULL DEFAULT 0,
    dns1 TEXT NOT NULL DEFAULT '',
    dns2 TEXT NOT NULL DEFAULT '',
    apply_status TEXT NOT NULL DEFAULT 'ready',
    apply_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tls_certificates_name ON of_tls_certificates (name);

CREATE TABLE IF NOT EXISTS of_waf_ip_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ip_list TEXT NOT NULL DEFAULT '[]',
    auto_config TEXT NOT NULL DEFAULT '{}',
    ext_ips TEXT NOT NULL DEFAULT '[]',
    subscription_url TEXT NOT NULL DEFAULT '',
    subscription_format TEXT NOT NULL DEFAULT 'text',
    subscription_mapping_rule TEXT NOT NULL DEFAULT '',
    sync_interval_minutes INTEGER NOT NULL DEFAULT 1440,
    last_synced_at DATETIME,
    next_sync_at DATETIME,
    last_sync_status TEXT NOT NULL DEFAULT '',
    last_sync_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_waf_ip_groups_next_sync_at ON of_waf_ip_groups (next_sync_at);
CREATE INDEX IF NOT EXISTS idx_of_waf_ip_groups_type ON of_waf_ip_groups (type);

CREATE TABLE IF NOT EXISTS of_waf_rule_group_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_group_id INTEGER NOT NULL,
    proxy_route_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sequence INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_waf_group_route ON of_waf_rule_group_bindings (rule_group_id, proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_waf_rule_group_bindings_proxy_route_id ON of_waf_rule_group_bindings (proxy_route_id);

CREATE TABLE IF NOT EXISTS of_waf_rule_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_global BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    graph TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_of_waf_rule_groups_is_global ON of_waf_rule_groups (is_global);

CREATE TABLE IF NOT EXISTS of_zone_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id INTEGER NOT NULL,
    proxy_route_id INTEGER,
    domain TEXT NOT NULL,
    cert_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_cert_id ON of_zone_domains (cert_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zone_domains_domain ON of_zone_domains (domain);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_proxy_route_id ON of_zone_domains (proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_zone_id ON of_zone_domains (zone_id);

CREATE TABLE IF NOT EXISTS of_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zones_domain ON of_zones (domain);

INSERT OR IGNORE INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES
    (101, 'OpenFlare SSL 自动续期', 'of_ssl_renew', '0 0 * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (103, 'OpenFlare WAF IP 组同步', 'of_waf_ip_group_sync', '*/5 * * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (104, 'OpenFlare Uptime Kuma 同步', 'of_uptime_kuma_sync', '* * * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (105, 'OpenFlare Pages 部署源扫描', 'of_pages_source_scan', '0 0 * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT OR IGNORE INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('agent_heartbeat_interval', '3000', 'business', 0, 'Agent 心跳间隔（毫秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('agent_update_repo', 'Rain-kl/OpenFlare', 'business', 0, 'Agent 更新仓库', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('agent_websocket_upgrade_enabled', 'true', 'business', 0, 'Agent WebSocket 升级开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('geoip_provider', 'ipinfo', 'business', 0, 'GeoIP 服务商', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_clickhouse', '30', 'business', 0, 'ClickHouse 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_postgres', '30', 'business', 0, 'PostgreSQL 日志保留天数（访问日志与可观测统一）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_sqlite', '30', 'business', 0, 'SQLite 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('metric_retention_days', '3', 'business', 0, '性能指标（CPU/内存/磁盘/网络）保留天数（三库共用）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('node_offline_threshold', '60000', 'business', 0, '节点离线阈值（毫秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_enabled', 'false', 'business', 0, '缓存开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_inactive', '30m', 'business', 0, '缓存不活跃时间', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_key_template', '$scheme$host$request_uri', 'business', 0, '缓存键模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_levels', '1:2', 'business', 0, '缓存层级', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_lock_enabled', 'true', 'business', 0, '缓存锁开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_lock_timeout', '5s', 'business', 0, '缓存锁超时', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_max_size', '1g', 'business', 0, '缓存最大大小', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_cache_use_stale', 'error timeout updating http_500 http_502 http_503 http_504', 'business', 0, '缓存失效策略', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_client_body_timeout', '15', 'business', 0, '客户端体超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_client_header_timeout', '15', 'business', 0, '客户端头超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_client_max_body_size', '64m', 'business', 0, '客户端最大体大小', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_default_limit_conn_per_ip', '0', 'business', 0, '默认单 IP 并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_default_limit_conn_per_server', '0', 'business', 0, '默认站点并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_default_limit_rate', '', 'business', 0, '默认单请求带宽限速（空关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_default_limit_req_per_ip', '', 'business', 0, '默认单 IP 请求频率限制（空关闭，例如 10r/s、100r/m）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_default_server_return_status', '421', 'business', 0, '默认服务器返回状态码', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_events_multi_accept_enabled', 'true', 'business', 0, '多路接受开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_events_use', 'epoll', 'business', 0, '事件模型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_gzip_comp_level', '5', 'business', 0, 'Gzip 压缩级别', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_gzip_enabled', 'true', 'business', 0, 'Gzip 压缩开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_gzip_min_length', '1024', 'business', 0, 'Gzip 最小长度', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_http3_enabled', 'true', 'business', 0, 'HTTP/3 支持开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_keepalive_requests', '1000', 'business', 0, 'Keepalive 请求数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_keepalive_timeout', '20', 'business', 0, 'Keepalive 超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_large_client_header_buffers', '4 16k', 'business', 0, '大客户端头缓冲区', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_buffer_size', '8k', 'business', 0, '代理缓冲区大小', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_buffering_enabled', 'true', 'business', 0, '代理响应缓冲开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_buffers', '16 16k', 'business', 0, '代理缓冲区', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_busy_buffers_size', '64k', 'business', 0, '代理繁忙缓冲区大小', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_connect_timeout', '3', 'business', 0, '代理连接超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_read_timeout', '60', 'business', 0, '代理读取超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_request_buffering_enabled', 'false', 'business', 0, '代理请求缓冲开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_proxy_send_timeout', '60', 'business', 0, '代理发送超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_send_timeout', '30', 'business', 0, '发送超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_websocket_enabled', 'true', 'business', 0, 'WebSocket 支持开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_worker_connections', '4096', 'business', 0, 'Worker 连接数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_worker_processes', 'auto', 'business', 0, 'Worker 进程数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('openresty_worker_rlimit_nofile', '65535', 'business', 0, 'Worker 文件描述符限制', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('origin_error_page_enabled', 'true', 'business', 0, '是否启用源站错误页', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('origin_error_page_get_only', 'false', 'business', 0, '源站错误页是否仅对 GET 请求生效（其它方法透传）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('origin_error_page_html', '', 'business', 0, '源站错误页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('origin_error_page_status_codes', '["500-599"]', 'business', 0, '源站错误页触发状态码标签 JSON 数组', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('pages_max_history_count', '20', 'business', 0, 'Pages 每个项目最大历史部署保留数（0 表示不限制）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('pages_max_package_size_mb', '100', 'business', 0, 'Pages 部署包上传大小上限（MiB）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('relay_frps_web_ui_enabled', 'false', 'business', 0, '是否启用 FRPS 内置 Web 管理界面', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('relay_frps_web_ui_port', '17500', 'business', 0, 'FRPS 内置 Web 管理界面端口', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('sw_offline_domains', '[]', 'business', 0, 'SW 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('sw_offline_enabled', 'false', 'business', 0, '是否启用 Service Worker 离线兜底', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('sw_offline_html', '', 'business', 0, '离线联系页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_enabled', 'false', 'business', 0, 'UptimeKuma 集成开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_interval', '60', 'business', 0, 'UptimeKuma 监控间隔（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_monitor_scope', 'all', 'business', 0, 'UptimeKuma 监控范围', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_retry', '0', 'business', 0, 'UptimeKuma 重试次数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_retry_interval', '60', 'business', 0, 'UptimeKuma 重试间隔（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_sync_interval', '5', 'business', 0, 'UptimeKuma 同步间隔（分钟）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('uptime_kuma_timeout', '48', 'business', 0, 'UptimeKuma 超时（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- +goose Down
SELECT 1;
