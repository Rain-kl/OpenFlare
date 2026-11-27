-- +goose Up
ALTER TABLE of_node_access_logs ADD COLUMN IF NOT EXISTS client_asn UInt32 DEFAULT 0;
ALTER TABLE of_node_access_logs ADD COLUMN IF NOT EXISTS tcp_rtt_us UInt32 DEFAULT 0;

CREATE TABLE IF NOT EXISTS of_network_quality_samples
(
    id             UInt64,
    node_id        String,
    node_asn       UInt32,
    client_asn     UInt32,
    client_country String,
    source         String,
    sample_count   Int32,
    rtt_min_ms     Float64,
    rtt_p50_ms     Float64,
    rtt_p95_ms     Float64,
    retrans_rate   Float64,
    loss_rate      Float64,
    hop_count      Int32,
    captured_at    DateTime64(3, 'UTC'),
    created_at     DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, client_asn, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY;

-- +goose Down
DROP TABLE IF EXISTS of_network_quality_samples;
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS tcp_rtt_us;
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS client_asn;
