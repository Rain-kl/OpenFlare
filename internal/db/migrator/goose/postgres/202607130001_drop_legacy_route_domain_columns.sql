-- +goose Up
-- 第二阶段：删除反代路由冗余域名/证书列与托管域名表。
-- 执行前必须完成 migrate-zones 且配置预览验收通过。

DROP INDEX IF EXISTS idx_of_proxy_routes_domain;

ALTER TABLE of_proxy_routes DROP COLUMN IF EXISTS domain;
ALTER TABLE of_proxy_routes DROP COLUMN IF EXISTS domains;
ALTER TABLE of_proxy_routes DROP COLUMN IF EXISTS cert_id;
ALTER TABLE of_proxy_routes DROP COLUMN IF EXISTS cert_ids;
ALTER TABLE of_proxy_routes DROP COLUMN IF EXISTS domain_cert_ids;

DROP TABLE IF EXISTS of_managed_domains;

-- +goose Down
-- 仅恢复开发库结构，不回填历史域名/证书数据。

CREATE TABLE IF NOT EXISTS of_managed_domains (
    id BIGSERIAL PRIMARY KEY,
    domain VARCHAR(255) NOT NULL,
    cert_id BIGINT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_managed_domains_domain ON of_managed_domains (domain);

ALTER TABLE of_proxy_routes ADD COLUMN IF NOT EXISTS domain VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE of_proxy_routes ADD COLUMN IF NOT EXISTS domains TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_proxy_routes ADD COLUMN IF NOT EXISTS cert_id BIGINT;
ALTER TABLE of_proxy_routes ADD COLUMN IF NOT EXISTS cert_ids TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_proxy_routes ADD COLUMN IF NOT EXISTS domain_cert_ids TEXT NOT NULL DEFAULT '[]';

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_proxy_routes_domain ON of_proxy_routes (domain);
