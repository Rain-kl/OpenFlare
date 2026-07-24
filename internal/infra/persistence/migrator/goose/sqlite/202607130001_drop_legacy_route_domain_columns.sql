-- +goose Up
-- 第二阶段：重建 of_proxy_routes（去掉冗余域名/证书列）并删除 of_managed_domains。

PRAGMA foreign_keys=OFF;

CREATE TABLE of_proxy_routes_new (
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
    remark TEXT NOT NULL DEFAULT '',
    upstream_type TEXT NOT NULL DEFAULT 'direct',
    tunnel_node_id INTEGER,
    tunnel_target_addr TEXT NOT NULL DEFAULT '',
    tunnel_target_protocol TEXT NOT NULL DEFAULT '',
    pages_project_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO of_proxy_routes_new (
    id, site_name, origin_id, origin_url, origin_host, upstreams, enabled,
    enable_https, redirect_http, limit_conn_per_server, limit_conn_per_ip, limit_rate,
    cache_enabled, cache_policy, cache_rules, custom_headers, basic_auth_enabled,
    basic_auth_username, basic_auth_password, remark, upstream_type, tunnel_node_id,
    tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at
)
SELECT
    id, site_name, origin_id, origin_url, origin_host, upstreams, enabled,
    enable_https, redirect_http, limit_conn_per_server, limit_conn_per_ip, limit_rate,
    cache_enabled, cache_policy, cache_rules, custom_headers, basic_auth_enabled,
    basic_auth_username, basic_auth_password, remark, upstream_type, tunnel_node_id,
    tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at
FROM of_proxy_routes;

DROP TABLE of_proxy_routes;
ALTER TABLE of_proxy_routes_new RENAME TO of_proxy_routes;

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_site_name ON of_proxy_routes (site_name);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_origin_id ON of_proxy_routes (origin_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_pages_project_id ON of_proxy_routes (pages_project_id);

DROP TABLE IF EXISTS of_managed_domains;

PRAGMA foreign_keys=ON;

-- +goose Down
-- 仅恢复开发库结构，不回填历史域名/证书数据。

PRAGMA foreign_keys=OFF;

CREATE TABLE IF NOT EXISTS of_managed_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    cert_id INTEGER,
    enabled INTEGER NOT NULL DEFAULT 1,
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_managed_domains_domain ON of_managed_domains (domain);

CREATE TABLE of_proxy_routes_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_name TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT '',
    domains TEXT NOT NULL DEFAULT '[]',
    origin_id INTEGER,
    origin_url TEXT NOT NULL,
    origin_host TEXT NOT NULL DEFAULT '',
    upstreams TEXT NOT NULL DEFAULT '[]',
    enabled INTEGER NOT NULL DEFAULT 1,
    enable_https INTEGER NOT NULL DEFAULT 0,
    cert_id INTEGER,
    cert_ids TEXT NOT NULL DEFAULT '[]',
    domain_cert_ids TEXT NOT NULL DEFAULT '[]',
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
    remark TEXT NOT NULL DEFAULT '',
    upstream_type TEXT NOT NULL DEFAULT 'direct',
    tunnel_node_id INTEGER,
    tunnel_target_addr TEXT NOT NULL DEFAULT '',
    tunnel_target_protocol TEXT NOT NULL DEFAULT '',
    pages_project_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO of_proxy_routes_old (
    id, site_name, domain, domains, origin_id, origin_url, origin_host, upstreams, enabled,
    enable_https, cert_id, cert_ids, domain_cert_ids, redirect_http, limit_conn_per_server,
    limit_conn_per_ip, limit_rate, cache_enabled, cache_policy, cache_rules, custom_headers,
    basic_auth_enabled, basic_auth_username, basic_auth_password, remark, upstream_type,
    tunnel_node_id, tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at
)
SELECT
    id, site_name, '', '[]', origin_id, origin_url, origin_host, upstreams, enabled,
    enable_https, NULL, '[]', '[]', redirect_http, limit_conn_per_server,
    limit_conn_per_ip, limit_rate, cache_enabled, cache_policy, cache_rules, custom_headers,
    basic_auth_enabled, basic_auth_username, basic_auth_password, remark, upstream_type,
    tunnel_node_id, tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at
FROM of_proxy_routes;

DROP TABLE of_proxy_routes;
ALTER TABLE of_proxy_routes_old RENAME TO of_proxy_routes;

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_domain ON of_proxy_routes (domain);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_site_name ON of_proxy_routes (site_name);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_origin_id ON of_proxy_routes (origin_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_pages_project_id ON of_proxy_routes (pages_project_id);

PRAGMA foreign_keys=ON;
