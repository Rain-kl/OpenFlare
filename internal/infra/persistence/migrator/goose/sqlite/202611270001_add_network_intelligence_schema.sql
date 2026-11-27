-- +goose Up
ALTER TABLE of_nodes ADD COLUMN asn INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN asn_org TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_nodes_asn ON of_nodes (asn);

ALTER TABLE of_node_access_logs ADD COLUMN client_asn INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_node_access_logs ADD COLUMN tcp_rtt_us INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_client_asn ON of_node_access_logs (client_asn, logged_at DESC);

CREATE TABLE IF NOT EXISTS of_network_quality_samples (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id        TEXT NOT NULL DEFAULT '',
    node_asn       INTEGER NOT NULL DEFAULT 0,
    client_asn     INTEGER NOT NULL DEFAULT 0,
    client_country TEXT NOT NULL DEFAULT '',
    source         TEXT NOT NULL DEFAULT 'tcp_passive',
    sample_count   INTEGER NOT NULL DEFAULT 0,
    rtt_min_ms     REAL NOT NULL DEFAULT 0,
    rtt_p50_ms     REAL NOT NULL DEFAULT 0,
    rtt_p95_ms     REAL NOT NULL DEFAULT 0,
    retrans_rate   REAL NOT NULL DEFAULT 0,
    loss_rate      REAL NOT NULL DEFAULT 0,
    hop_count      INTEGER NOT NULL DEFAULT 0,
    captured_at    DATETIME NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_node_asn_time ON of_network_quality_samples (node_id, client_asn, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_client_asn_time ON of_network_quality_samples (client_asn, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_captured_at ON of_network_quality_samples (captured_at DESC);

CREATE TABLE IF NOT EXISTS of_network_quality_matrix (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id            TEXT NOT NULL,
    node_asn           INTEGER NOT NULL DEFAULT 0,
    client_asn         INTEGER NOT NULL,
    client_country     TEXT NOT NULL DEFAULT '',
    client_asn_name    TEXT NOT NULL DEFAULT '',
    tcp_rtt_p50_ms     REAL NOT NULL DEFAULT 0,
    tcp_rtt_p95_ms     REAL NOT NULL DEFAULT 0,
    tcp_min_rtt_ms     REAL NOT NULL DEFAULT 0,
    tcp_retrans_rate   REAL NOT NULL DEFAULT 0,
    tcp_sample_count   INTEGER NOT NULL DEFAULT 0,
    tcp_last_seen_at   DATETIME NULL,
    trace_rtt_p50_ms   REAL NOT NULL DEFAULT 0,
    trace_loss_rate    REAL NOT NULL DEFAULT 0,
    trace_hop_count    INTEGER NOT NULL DEFAULT 0,
    trace_as_path      TEXT NOT NULL DEFAULT '[]',
    trace_sample_count INTEGER NOT NULL DEFAULT 0,
    trace_last_seen_at DATETIME NULL,
    effective_rtt_ms   REAL NOT NULL DEFAULT 0,
    predicted_rtt_ms   REAL NOT NULL DEFAULT 0,
    quality_score      REAL NOT NULL DEFAULT 0,
    confidence         REAL NOT NULL DEFAULT 0,
    data_source        TEXT NOT NULL DEFAULT 'measured',
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_nq_matrix_node_asn ON of_network_quality_matrix (node_id, client_asn);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_client_asn ON of_network_quality_matrix (client_asn);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_country ON of_network_quality_matrix (client_country);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_score ON of_network_quality_matrix (quality_score DESC);

CREATE TABLE IF NOT EXISTS of_as_topology_edges (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    src_asn        INTEGER NOT NULL,
    dst_asn        INTEGER NOT NULL,
    edge_type      TEXT NOT NULL DEFAULT 'observed',
    avg_hop_rtt_ms REAL NOT NULL DEFAULT 0,
    min_hop_rtt_ms REAL NOT NULL DEFAULT 0,
    sample_count   INTEGER NOT NULL DEFAULT 0,
    last_seen_at   DATETIME NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_as_edges_pair ON of_as_topology_edges (src_asn, dst_asn);
CREATE INDEX IF NOT EXISTS idx_of_as_edges_dst ON of_as_topology_edges (dst_asn);

CREATE TABLE IF NOT EXISTS of_traceroute_records (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id        TEXT NOT NULL DEFAULT '',
    trigger_type   TEXT NOT NULL DEFAULT 'manual',
    node_id        TEXT NOT NULL,
    target         TEXT NOT NULL,
    resolved_ip    TEXT NOT NULL DEFAULT '',
    target_asn     INTEGER NOT NULL DEFAULT 0,
    target_country TEXT NOT NULL DEFAULT '',
    protocol       TEXT NOT NULL DEFAULT 'icmp',
    hop_count      INTEGER NOT NULL DEFAULT 0,
    reached        INTEGER NOT NULL DEFAULT 0,
    total_rtt_ms   REAL NOT NULL DEFAULT 0,
    hops_json      TEXT NOT NULL DEFAULT '[]',
    as_path_json   TEXT NOT NULL DEFAULT '[]',
    error_message  TEXT NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_traceroute_task ON of_traceroute_records (task_id);
CREATE INDEX IF NOT EXISTS idx_of_traceroute_node_time ON of_traceroute_records (node_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_traceroute_target_asn ON of_traceroute_records (target_asn, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_traceroute_created_at ON of_traceroute_records (created_at);

-- +goose Down
DROP TABLE IF EXISTS of_traceroute_records;
DROP TABLE IF EXISTS of_as_topology_edges;
DROP TABLE IF EXISTS of_network_quality_matrix;
DROP TABLE IF EXISTS of_network_quality_samples;
DROP INDEX IF EXISTS idx_of_node_access_logs_client_asn;
ALTER TABLE of_node_access_logs DROP COLUMN tcp_rtt_us;
ALTER TABLE of_node_access_logs DROP COLUMN client_asn;
DROP INDEX IF EXISTS idx_of_nodes_asn;
ALTER TABLE of_nodes DROP COLUMN asn_org;
ALTER TABLE of_nodes DROP COLUMN asn;
