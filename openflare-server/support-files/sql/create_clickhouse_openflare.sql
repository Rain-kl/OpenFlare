CREATE DATABASE IF NOT EXISTS openflare;

USE openflare;

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

CREATE TABLE IF NOT EXISTS w_user_access_logs
(
    id          UInt64,
    user_id     UInt64,
    path        String,
    method      String,
    ip          String,
    user_agent  String,
    headers     String,
    status      Int32,
    latency     Int64,
    created_at  DateTime
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (created_at, ip, user_id)
SETTINGS index_granularity = 8192;