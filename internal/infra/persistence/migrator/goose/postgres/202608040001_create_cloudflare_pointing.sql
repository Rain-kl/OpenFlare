-- +goose Up
CREATE TABLE IF NOT EXISTS of_cf_connections (
    id BIGSERIAL PRIMARY KEY,
    source VARCHAR(32) NOT NULL DEFAULT '',
    dns_account_id BIGINT,
    "authorization" TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT '',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_connections_dns_account_id ON of_cf_connections (dns_account_id);

CREATE TABLE IF NOT EXISTS of_cf_pointing_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    primary_node_id BIGINT NOT NULL,
    backup_node_id BIGINT,
    active_node_id BIGINT NOT NULL,
    default_proxied BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_primary_node_id ON of_cf_pointing_groups (primary_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_backup_node_id ON of_cf_pointing_groups (backup_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_active_node_id ON of_cf_pointing_groups (active_node_id);

CREATE TABLE IF NOT EXISTS of_cf_pointing_members (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    zone_domain_id BIGINT NOT NULL,
    proxied BOOLEAN NOT NULL DEFAULT FALSE,
    cf_zone_id VARCHAR(64) NOT NULL DEFAULT '',
    cf_record_id VARCHAR(64) NOT NULL DEFAULT '',
    desired_ip VARCHAR(64) NOT NULL DEFAULT '',
    sync_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT '',
    synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_members_group_id ON of_cf_pointing_members (group_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_cf_pointing_members_zone_domain_id ON of_cf_pointing_members (zone_domain_id);

-- +goose Down
DROP TABLE IF EXISTS of_cf_pointing_members;
DROP TABLE IF EXISTS of_cf_pointing_groups;
DROP TABLE IF EXISTS of_cf_connections;
