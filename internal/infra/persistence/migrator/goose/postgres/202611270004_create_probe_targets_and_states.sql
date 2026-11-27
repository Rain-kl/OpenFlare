-- +goose Up
CREATE TABLE IF NOT EXISTS of_probe_targets (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(128) NOT NULL DEFAULT '',
    ip             VARCHAR(64) NOT NULL,
    ip_family      VARCHAR(8) NOT NULL DEFAULT 'ipv4',
    probe_proto    VARCHAR(16) NOT NULL DEFAULT 'icmp',
    port           INTEGER NOT NULL DEFAULT 0,
    asn            BIGINT NOT NULL DEFAULT 0,
    country        VARCHAR(16) NOT NULL DEFAULT '',
    region         VARCHAR(32) NOT NULL DEFAULT '',
    target_type    VARCHAR(16) NOT NULL DEFAULT 'public_anchor',
    anycast_status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    notes          VARCHAR(255) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_probe_targets_ip_proto_port ON of_probe_targets (ip, probe_proto, port);
CREATE INDEX IF NOT EXISTS idx_of_probe_targets_asn_enabled ON of_probe_targets (asn, enabled);
CREATE INDEX IF NOT EXISTS idx_of_probe_targets_geo ON of_probe_targets (country, region);

CREATE TABLE IF NOT EXISTS of_probe_target_states (
    id                   BIGSERIAL PRIMARY KEY,
    node_id              VARCHAR(64) NOT NULL,
    target_id            BIGINT NOT NULL,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_probed_at       TIMESTAMPTZ NULL,
    last_success_at      TIMESTAMPTZ NULL,
    next_probe_at        TIMESTAMPTZ NULL,
    last_error           TEXT NOT NULL DEFAULT '',
    last_rtt_ms          DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_probe_target_states_node_target ON of_probe_target_states (node_id, target_id);
CREATE INDEX IF NOT EXISTS idx_of_probe_target_states_next_probe ON of_probe_target_states (next_probe_at, target_id);

-- +goose Down
DROP TABLE IF EXISTS of_probe_target_states;
DROP TABLE IF EXISTS of_probe_targets;
