-- +goose Up
CREATE TABLE IF NOT EXISTS of_proxy_routes (
    id BIGSERIAL PRIMARY KEY,
    site_name VARCHAR(255) NOT NULL DEFAULT '',
    domain VARCHAR(255) NOT NULL,
    domains TEXT NOT NULL DEFAULT '[]',
    origin_id BIGINT,
    origin_url VARCHAR(2048) NOT NULL,
    origin_host VARCHAR(255) NOT NULL DEFAULT '',
    upstreams TEXT NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    enable_https BOOLEAN NOT NULL DEFAULT FALSE,
    cert_id BIGINT,
    cert_ids TEXT NOT NULL DEFAULT '[]',
    domain_cert_ids TEXT NOT NULL DEFAULT '[]',
    redirect_http BOOLEAN NOT NULL DEFAULT FALSE,
    limit_conn_per_server INTEGER NOT NULL DEFAULT 0,
    limit_conn_per_ip INTEGER NOT NULL DEFAULT 0,
    limit_rate VARCHAR(32) NOT NULL DEFAULT '',
    cache_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    cache_policy VARCHAR(32) NOT NULL DEFAULT '',
    cache_rules TEXT NOT NULL DEFAULT '[]',
    custom_headers TEXT NOT NULL DEFAULT '[]',
    basic_auth_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    basic_auth_username VARCHAR(255) NOT NULL DEFAULT '',
    basic_auth_password VARCHAR(255) NOT NULL DEFAULT '',
    remark VARCHAR(255) NOT NULL DEFAULT '',
    upstream_type VARCHAR(32) NOT NULL DEFAULT 'direct',
    tunnel_node_id BIGINT,
    tunnel_target_addr VARCHAR(512) NOT NULL DEFAULT '',
    tunnel_target_protocol VARCHAR(16) NOT NULL DEFAULT '',
    pages_project_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_domain ON of_proxy_routes (domain);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_site_name ON of_proxy_routes (site_name);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_origin_id ON of_proxy_routes (origin_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_tunnel_node_id ON of_proxy_routes (tunnel_node_id);
CREATE INDEX IF NOT EXISTS idx_of_proxy_routes_pages_project_id ON of_proxy_routes (pages_project_id);

-- +goose Down
DROP TABLE IF EXISTS of_proxy_routes;