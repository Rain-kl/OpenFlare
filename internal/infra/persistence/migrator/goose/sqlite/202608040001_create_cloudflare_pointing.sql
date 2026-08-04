-- +goose Up
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
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_primary_node_id ON of_cf_pointing_groups (primary_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_backup_node_id ON of_cf_pointing_groups (backup_node_id);
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_active_node_id ON of_cf_pointing_groups (active_node_id);

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

-- +goose Down
DROP TABLE IF EXISTS of_cf_pointing_members;
DROP TABLE IF EXISTS of_cf_pointing_groups;
DROP TABLE IF EXISTS of_cf_connections;
