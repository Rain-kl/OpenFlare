-- +goose Up
CREATE TABLE IF NOT EXISTS of_dns_metric_snapshots
(
    id                  UInt64,
    instance_id         String,
    captured_at         DateTime64(3, 'UTC'),
    queries_total       UInt64,
    queries_success     UInt64,
    queries_nxdomain    UInt64,
    queries_servfail    UInt64,
    queries_ratelimited UInt64,
    qps                 Float64,
    latency_p50_ms      Float64,
    latency_p95_ms      Float64,
    latency_p99_ms      Float64,
    cache_hit_ratio     Float64,
    snapshot_version    UInt64,
    cpu                 Float64,
    ram_percent         Float64,
    created_at          DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (instance_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY;

-- +goose Down
DROP TABLE IF EXISTS of_dns_metric_snapshots;
