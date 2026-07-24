-- +goose Up
CREATE TABLE IF NOT EXISTS of_proxy_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_name TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL,
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_domain ON of_proxy_routes (domain);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_site_name ON of_proxy_routes (site_name);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_origin_id ON of_proxy_routes (origin_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_pages_project_id ON of_proxy_routes (pages_project_id);

-- +goose Down
DROP TABLE IF EXISTS of_proxy_routes;