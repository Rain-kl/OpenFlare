-- +goose Up
CREATE TABLE IF NOT EXISTS of_waf_rule_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_global BOOLEAN NOT NULL DEFAULT FALSE,
    block_status_code INTEGER NOT NULL DEFAULT 418,
    block_response_body TEXT NOT NULL DEFAULT '',
    ip_whitelist TEXT NOT NULL DEFAULT '[]',
    ip_blacklist TEXT NOT NULL DEFAULT '[]',
    ip_whitelist_groups TEXT NOT NULL DEFAULT '[]',
    ip_blacklist_groups TEXT NOT NULL DEFAULT '[]',
    country_whitelist TEXT NOT NULL DEFAULT '[]',
    country_blacklist TEXT NOT NULL DEFAULT '[]',
    region_whitelist TEXT NOT NULL DEFAULT '[]',
    region_blacklist TEXT NOT NULL DEFAULT '[]',
    pow_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    pow_config TEXT NOT NULL DEFAULT '{}',
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_waf_rule_groups_is_global ON of_waf_rule_groups (is_global);

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
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_waf_ip_groups_type ON of_waf_ip_groups (type);
CREATE INDEX IF NOT EXISTS idx_of_waf_ip_groups_next_sync_at ON of_waf_ip_groups (next_sync_at);

CREATE TABLE IF NOT EXISTS of_waf_rule_group_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_group_id INTEGER NOT NULL,
    proxy_route_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_waf_group_route ON of_waf_rule_group_bindings (rule_group_id, proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_waf_rule_group_bindings_proxy_route_id ON of_waf_rule_group_bindings (proxy_route_id);

-- +goose Down
DROP TABLE IF EXISTS of_waf_rule_group_bindings;
DROP TABLE IF EXISTS of_waf_ip_groups;
DROP TABLE IF EXISTS of_waf_rule_groups;