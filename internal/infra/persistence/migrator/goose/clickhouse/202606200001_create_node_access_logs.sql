-- +goose Up
CREATE TABLE IF NOT EXISTS of_node_access_logs
(
    id          UInt64,
    node_id     String,
    logged_at   DateTime64(3, 'UTC'),
    remote_addr String,
    region      String,
    host        String,
    path        String,
    status_code Int32,
    created_at  DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(logged_at)
ORDER BY (node_id, logged_at, remote_addr, host, path, status_code)
SETTINGS index_granularity = 8192;

-- +goose Down
DROP TABLE IF EXISTS of_node_access_logs;