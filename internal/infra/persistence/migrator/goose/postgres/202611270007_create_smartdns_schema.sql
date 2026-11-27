-- +goose Up
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS capacity_bandwidth_mbps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS capacity_connections INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS capacity_weight INTEGER NOT NULL DEFAULT 100;

CREATE TABLE IF NOT EXISTS of_node_tags (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(64) NOT NULL,
    slug           VARCHAR(64) NOT NULL,
    color          VARCHAR(32) NOT NULL DEFAULT 'blue',
    description    VARCHAR(255) NOT NULL DEFAULT '',
    default_weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_node_tags_slug ON of_node_tags (slug);

CREATE TABLE IF NOT EXISTS of_node_tag_bindings (
    id              BIGSERIAL PRIMARY KEY,
    node_id         VARCHAR(64) NOT NULL,
    tag_id          BIGINT NOT NULL,
    weight_override DOUBLE PRECISION NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_node_tag_bindings_pair ON of_node_tag_bindings (node_id, tag_id);
CREATE INDEX IF NOT EXISTS idx_of_node_tag_bindings_tag ON of_node_tag_bindings (tag_id);

CREATE TABLE IF NOT EXISTS of_dns_instances (
    id                   BIGSERIAL PRIMARY KEY,
    instance_id          VARCHAR(64) NOT NULL,
    name                 VARCHAR(128) NOT NULL,
    mode                 VARCHAR(16) NOT NULL DEFAULT 'standalone',
    public_addr          VARCHAR(128) NOT NULL DEFAULT '',
    listen_addr          VARCHAR(64) NOT NULL DEFAULT ':53',
    geo_name             VARCHAR(128) NOT NULL DEFAULT '',
    geo_latitude         DOUBLE PRECISION NULL,
    geo_longitude        DOUBLE PRECISION NULL,
    access_token         VARCHAR(128) NOT NULL DEFAULT '',
    version              VARCHAR(64) NOT NULL DEFAULT '',
    status               VARCHAR(16) NOT NULL DEFAULT 'pending',
    snapshot_version     BIGINT NOT NULL DEFAULT 0,
    snapshot_etag        VARCHAR(64) NOT NULL DEFAULT '',
    qps                  DOUBLE PRECISION NOT NULL DEFAULT 0,
    p50_ms               DOUBLE PRECISION NOT NULL DEFAULT 0,
    p95_ms               DOUBLE PRECISION NOT NULL DEFAULT 0,
    p99_ms               DOUBLE PRECISION NOT NULL DEFAULT 0,
    success_total        BIGINT NOT NULL DEFAULT 0,
    error_total          BIGINT NOT NULL DEFAULT 0,
    cache_hit_ratio      DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_usage_percent    DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_seen_at         TIMESTAMPTZ NULL,
    last_error           TEXT NOT NULL DEFAULT '',
    quarantined_at       TIMESTAMPTZ NULL,
    quarantine_reason    VARCHAR(255) NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_dns_instances_instance_id ON of_dns_instances (instance_id);
CREATE INDEX IF NOT EXISTS idx_of_dns_instances_token ON of_dns_instances (access_token);

CREATE TABLE IF NOT EXISTS of_dns_policies (
    id                      BIGSERIAL PRIMARY KEY,
    name                    VARCHAR(128) NOT NULL,
    description             VARCHAR(255) NOT NULL DEFAULT '',
    scope_type              VARCHAR(16) NOT NULL DEFAULT 'all',
    scope_domains_json      TEXT NOT NULL DEFAULT '[]',
    strategy_type           VARCHAR(24) NOT NULL DEFAULT 'smart',
    priority                INTEGER NOT NULL DEFAULT 100,
    ttl                     INTEGER NOT NULL DEFAULT 60,
    max_answers             INTEGER NOT NULL DEFAULT 1,
    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    is_default              BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_strategy       VARCHAR(24) NOT NULL DEFAULT 'smart',
    fallback_node_ids_json  TEXT NOT NULL DEFAULT '[]',
    config_json             TEXT NOT NULL DEFAULT '{}',
    jdm_json                TEXT NOT NULL DEFAULT '{}',
    revision                INTEGER NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_dns_policies_priority ON of_dns_policies (enabled, priority ASC, id ASC);

ALTER TABLE of_cf_pointing_groups ADD COLUMN IF NOT EXISTS smartdns_policy_id BIGINT NULL;
ALTER TABLE of_cf_pointing_groups ADD COLUMN IF NOT EXISTS smartdns_target_host VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_cf_pointing_groups_smartdns_policy ON of_cf_pointing_groups (smartdns_policy_id);

CREATE TABLE IF NOT EXISTS of_dns_metric_snapshots (
    id                  BIGINT PRIMARY KEY,
    instance_id         VARCHAR(64) NOT NULL DEFAULT '',
    captured_at         TIMESTAMPTZ NOT NULL,
    queries_total       BIGINT NOT NULL DEFAULT 0,
    queries_success     BIGINT NOT NULL DEFAULT 0,
    queries_nxdomain    BIGINT NOT NULL DEFAULT 0,
    queries_servfail    BIGINT NOT NULL DEFAULT 0,
    queries_ratelimited BIGINT NOT NULL DEFAULT 0,
    qps                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_p50_ms      DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_p95_ms      DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_p99_ms      DOUBLE PRECISION NOT NULL DEFAULT 0,
    cache_hit_ratio     DOUBLE PRECISION NOT NULL DEFAULT 0,
    snapshot_version    BIGINT NOT NULL DEFAULT 0,
    cpu                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    ram_percent         DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_dns_metrics_instance_time ON of_dns_metric_snapshots (instance_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_dns_metrics_captured_at ON of_dns_metric_snapshots (captured_at DESC);

-- +goose Down
DROP TABLE IF EXISTS of_dns_metric_snapshots;
DROP INDEX IF EXISTS idx_of_cf_pointing_groups_smartdns_policy;
ALTER TABLE of_cf_pointing_groups DROP COLUMN IF EXISTS smartdns_target_host;
ALTER TABLE of_cf_pointing_groups DROP COLUMN IF EXISTS smartdns_policy_id;
DROP TABLE IF EXISTS of_dns_policies;
DROP TABLE IF EXISTS of_dns_instances;
DROP TABLE IF EXISTS of_node_tag_bindings;
DROP TABLE IF EXISTS of_node_tags;
ALTER TABLE of_nodes DROP COLUMN IF EXISTS capacity_weight;
ALTER TABLE of_nodes DROP COLUMN IF EXISTS capacity_connections;
ALTER TABLE of_nodes DROP COLUMN IF EXISTS capacity_bandwidth_mbps;
