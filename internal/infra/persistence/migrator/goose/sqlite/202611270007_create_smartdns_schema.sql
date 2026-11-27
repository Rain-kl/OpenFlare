-- +goose Up
ALTER TABLE of_nodes ADD COLUMN capacity_bandwidth_mbps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN capacity_connections INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN capacity_weight INTEGER NOT NULL DEFAULT 100;

CREATE TABLE IF NOT EXISTS of_node_tags (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL,
    slug           TEXT NOT NULL,
    color          TEXT NOT NULL DEFAULT 'blue',
    description    TEXT NOT NULL DEFAULT '',
    default_weight REAL NOT NULL DEFAULT 1.0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_node_tags_slug ON of_node_tags (slug);

CREATE TABLE IF NOT EXISTS of_node_tag_bindings (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id         TEXT NOT NULL,
    tag_id          INTEGER NOT NULL,
    weight_override REAL NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_node_tag_bindings_pair ON of_node_tag_bindings (node_id, tag_id);
CREATE INDEX IF NOT EXISTS idx_of_node_tag_bindings_tag ON of_node_tag_bindings (tag_id);

CREATE TABLE IF NOT EXISTS of_dns_instances (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id          TEXT NOT NULL,
    name                 TEXT NOT NULL,
    mode                 TEXT NOT NULL DEFAULT 'standalone',
    public_addr          TEXT NOT NULL DEFAULT '',
    listen_addr          TEXT NOT NULL DEFAULT ':53',
    geo_name             TEXT NOT NULL DEFAULT '',
    geo_latitude         REAL NULL,
    geo_longitude        REAL NULL,
    access_token         TEXT NOT NULL DEFAULT '',
    version              TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'pending',
    snapshot_version     INTEGER NOT NULL DEFAULT 0,
    snapshot_etag        TEXT NOT NULL DEFAULT '',
    qps                  REAL NOT NULL DEFAULT 0,
    p50_ms               REAL NOT NULL DEFAULT 0,
    p95_ms               REAL NOT NULL DEFAULT 0,
    p99_ms               REAL NOT NULL DEFAULT 0,
    success_total        INTEGER NOT NULL DEFAULT 0,
    error_total          INTEGER NOT NULL DEFAULT 0,
    cache_hit_ratio      REAL NOT NULL DEFAULT 0,
    cpu_usage_percent    REAL NOT NULL DEFAULT 0,
    memory_usage_percent REAL NOT NULL DEFAULT 0,
    last_seen_at         DATETIME NULL,
    last_error           TEXT NOT NULL DEFAULT '',
    quarantined_at       DATETIME NULL,
    quarantine_reason    TEXT NOT NULL DEFAULT '',
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_dns_instances_instance_id ON of_dns_instances (instance_id);
CREATE INDEX IF NOT EXISTS idx_of_dns_instances_token ON of_dns_instances (access_token);

CREATE TABLE IF NOT EXISTS of_dns_policies (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    name                    TEXT NOT NULL,
    description             TEXT NOT NULL DEFAULT '',
    scope_type              TEXT NOT NULL DEFAULT 'all',
    scope_domains_json      TEXT NOT NULL DEFAULT '[]',
    strategy_type           TEXT NOT NULL DEFAULT 'smart',
    priority                INTEGER NOT NULL DEFAULT 100,
    ttl                     INTEGER NOT NULL DEFAULT 60,
    max_answers             INTEGER NOT NULL DEFAULT 1,
    enabled                 INTEGER NOT NULL DEFAULT 1,
    is_default              INTEGER NOT NULL DEFAULT 0,
    fallback_strategy       TEXT NOT NULL DEFAULT 'smart',
    fallback_node_ids_json  TEXT NOT NULL DEFAULT '[]',
    config_json             TEXT NOT NULL DEFAULT '{}',
    jdm_json                TEXT NOT NULL DEFAULT '{}',
    revision                INTEGER NOT NULL DEFAULT 1,
    created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_dns_policies_priority ON of_dns_policies (enabled, priority ASC, id ASC);

ALTER TABLE of_cf_pointing_groups ADD COLUMN smartdns_policy_id INTEGER NULL;
ALTER TABLE of_cf_pointing_groups ADD COLUMN smartdns_target_host TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_smartdns_policy ON of_cf_pointing_groups (smartdns_policy_id);

CREATE TABLE IF NOT EXISTS of_dns_metric_snapshots (
    id                  INTEGER PRIMARY KEY,
    instance_id         TEXT NOT NULL DEFAULT '',
    captured_at         DATETIME NOT NULL,
    queries_total       INTEGER NOT NULL DEFAULT 0,
    queries_success     INTEGER NOT NULL DEFAULT 0,
    queries_nxdomain    INTEGER NOT NULL DEFAULT 0,
    queries_servfail    INTEGER NOT NULL DEFAULT 0,
    queries_ratelimited INTEGER NOT NULL DEFAULT 0,
    qps                 REAL NOT NULL DEFAULT 0,
    latency_p50_ms      REAL NOT NULL DEFAULT 0,
    latency_p95_ms      REAL NOT NULL DEFAULT 0,
    latency_p99_ms      REAL NOT NULL DEFAULT 0,
    cache_hit_ratio     REAL NOT NULL DEFAULT 0,
    snapshot_version    INTEGER NOT NULL DEFAULT 0,
    cpu                 REAL NOT NULL DEFAULT 0,
    ram_percent         REAL NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_dns_metrics_instance_time ON of_dns_metric_snapshots (instance_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_dns_metrics_captured_at ON of_dns_metric_snapshots (captured_at DESC);

-- +goose Down
DROP TABLE IF EXISTS of_dns_metric_snapshots;
DROP INDEX IF EXISTS idx_of_cf_pointing_groups_smartdns_policy;
ALTER TABLE of_cf_pointing_groups DROP COLUMN smartdns_target_host;
ALTER TABLE of_cf_pointing_groups DROP COLUMN smartdns_policy_id;
DROP TABLE IF EXISTS of_dns_policies;
DROP TABLE IF EXISTS of_dns_instances;
DROP TABLE IF EXISTS of_node_tag_bindings;
DROP TABLE IF EXISTS of_node_tags;
ALTER TABLE of_nodes DROP COLUMN capacity_weight;
ALTER TABLE of_nodes DROP COLUMN capacity_connections;
ALTER TABLE of_nodes DROP COLUMN capacity_bandwidth_mbps;
