-- +goose Up
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS asn BIGINT NOT NULL DEFAULT 0;
ALTER TABLE of_nodes ADD COLUMN IF NOT EXISTS asn_org VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_nodes_asn ON of_nodes (asn);

ALTER TABLE of_node_access_logs ADD COLUMN IF NOT EXISTS client_asn BIGINT NOT NULL DEFAULT 0;
ALTER TABLE of_node_access_logs ADD COLUMN IF NOT EXISTS tcp_rtt_us INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_client_asn ON of_node_access_logs (client_asn, logged_at DESC);

CREATE TABLE IF NOT EXISTS of_network_quality_samples (
    id             BIGINT PRIMARY KEY,
    node_id        VARCHAR(64) NOT NULL DEFAULT '',
    node_asn       BIGINT NOT NULL DEFAULT 0,
    client_asn     BIGINT NOT NULL DEFAULT 0,
    client_country VARCHAR(16) NOT NULL DEFAULT '',
    source         VARCHAR(16) NOT NULL DEFAULT 'tcp_passive',
    sample_count   INTEGER NOT NULL DEFAULT 0,
    rtt_min_ms     DOUBLE PRECISION NOT NULL DEFAULT 0,
    rtt_p50_ms     DOUBLE PRECISION NOT NULL DEFAULT 0,
    rtt_p95_ms     DOUBLE PRECISION NOT NULL DEFAULT 0,
    retrans_rate   DOUBLE PRECISION NOT NULL DEFAULT 0,
    loss_rate      DOUBLE PRECISION NOT NULL DEFAULT 0,
    hop_count      INTEGER NOT NULL DEFAULT 0,
    captured_at    TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_node_asn_time ON of_network_quality_samples (node_id, client_asn, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_client_asn_time ON of_network_quality_samples (client_asn, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_nq_samples_captured_at ON of_network_quality_samples (captured_at DESC);

CREATE TABLE IF NOT EXISTS of_network_quality_matrix (
    id                BIGSERIAL PRIMARY KEY,
    node_id           VARCHAR(64) NOT NULL,
    node_asn          BIGINT NOT NULL DEFAULT 0,
    client_asn        BIGINT NOT NULL,
    client_country    VARCHAR(16) NOT NULL DEFAULT '',
    client_asn_name   VARCHAR(255) NOT NULL DEFAULT '',
    tcp_rtt_p50_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,
    tcp_rtt_p95_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,
    tcp_min_rtt_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,
    tcp_retrans_rate  DOUBLE PRECISION NOT NULL DEFAULT 0,
    tcp_sample_count  INTEGER NOT NULL DEFAULT 0,
    tcp_last_seen_at  TIMESTAMPTZ NULL,
    trace_rtt_p50_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    trace_loss_rate   DOUBLE PRECISION NOT NULL DEFAULT 0,
    trace_hop_count   INTEGER NOT NULL DEFAULT 0,
    trace_as_path     TEXT NOT NULL DEFAULT '[]',
    trace_sample_count INTEGER NOT NULL DEFAULT 0,
    trace_last_seen_at TIMESTAMPTZ NULL,
    effective_rtt_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    predicted_rtt_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    quality_score     DOUBLE PRECISION NOT NULL DEFAULT 0,
    confidence        DOUBLE PRECISION NOT NULL DEFAULT 0,
    data_source       VARCHAR(16) NOT NULL DEFAULT 'measured',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_nq_matrix_node_asn ON of_network_quality_matrix (node_id, client_asn);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_client_asn ON of_network_quality_matrix (client_asn);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_country ON of_network_quality_matrix (client_country);
CREATE INDEX IF NOT EXISTS idx_of_nq_matrix_score ON of_network_quality_matrix (quality_score DESC);

CREATE TABLE IF NOT EXISTS of_as_topology_edges (
    id             BIGSERIAL PRIMARY KEY,
    src_asn        BIGINT NOT NULL,
    dst_asn        BIGINT NOT NULL,
    edge_type      VARCHAR(16) NOT NULL DEFAULT 'observed',
    avg_hop_rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    min_hop_rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    sample_count   INTEGER NOT NULL DEFAULT 0,
    last_seen_at   TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_of_as_edges_pair ON of_as_topology_edges (src_asn, dst_asn);
CREATE INDEX IF NOT EXISTS idx_of_as_edges_dst ON of_as_topology_edges (dst_asn);

CREATE TABLE IF NOT EXISTS of_traceroute_records (
    id             BIGSERIAL PRIMARY KEY,
    task_id        VARCHAR(64) NOT NULL DEFAULT '',
    trigger_type   VARCHAR(16) NOT NULL DEFAULT 'manual',
    node_id        VARCHAR(64) NOT NULL,
    target         VARCHAR(255) NOT NULL,
    resolved_ip    VARCHAR(64) NOT NULL DEFAULT '',
    target_asn     BIGINT NOT NULL DEFAULT 0,
    target_country VARCHAR(16) NOT NULL DEFAULT '',
    protocol       VARCHAR(16) NOT NULL DEFAULT 'icmp',
    hop_count      INTEGER NOT NULL DEFAULT 0,
    reached        BOOLEAN NOT NULL DEFAULT FALSE,
    total_rtt_ms   DOUBLE PRECISION NOT NULL DEFAULT 0,
    hops_json      TEXT NOT NULL DEFAULT '[]',
    as_path_json   TEXT NOT NULL DEFAULT '[]',
    error_message  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS tcp_rtt_us;
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS client_asn;
DROP INDEX IF EXISTS idx_of_nodes_asn;
ALTER TABLE of_nodes DROP COLUMN IF EXISTS asn_org;
ALTER TABLE of_nodes DROP COLUMN IF EXISTS asn;
