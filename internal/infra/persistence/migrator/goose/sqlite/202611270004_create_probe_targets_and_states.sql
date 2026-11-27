-- +goose Up
CREATE TABLE IF NOT EXISTS of_probe_targets (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL DEFAULT '',
    ip             TEXT NOT NULL,
    ip_family      TEXT NOT NULL DEFAULT 'ipv4',
    probe_proto    TEXT NOT NULL DEFAULT 'icmp',
    port           INTEGER NOT NULL DEFAULT 0,
    asn            INTEGER NOT NULL DEFAULT 0,
    country        TEXT NOT NULL DEFAULT '',
    region         TEXT NOT NULL DEFAULT '',
    target_type    TEXT NOT NULL DEFAULT 'public_anchor',
    anycast_status TEXT NOT NULL DEFAULT 'unknown',
    enabled        INTEGER NOT NULL DEFAULT 1,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_probe_targets_ip_proto_port ON of_probe_targets (ip, probe_proto, port);
CREATE INDEX IF NOT EXISTS idx_of_probe_targets_asn_enabled ON of_probe_targets (asn, enabled);
CREATE INDEX IF NOT EXISTS idx_of_probe_targets_geo ON of_probe_targets (country, region);

CREATE TABLE IF NOT EXISTS of_probe_target_states (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id              TEXT NOT NULL,
    target_id            INTEGER NOT NULL,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_probed_at       DATETIME NULL,
    last_success_at      DATETIME NULL,
    next_probe_at        DATETIME NULL,
    last_error           TEXT NOT NULL DEFAULT '',
    last_rtt_ms          REAL NOT NULL DEFAULT 0,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_probe_target_states_node_target ON of_probe_target_states (node_id, target_id);
CREATE INDEX IF NOT EXISTS idx_of_probe_target_states_next_probe ON of_probe_target_states (next_probe_at, target_id);

-- +goose Down
DROP TABLE IF EXISTS of_probe_target_states;
DROP TABLE IF EXISTS of_probe_targets;
