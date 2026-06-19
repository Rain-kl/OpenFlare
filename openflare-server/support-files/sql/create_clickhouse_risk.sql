CREATE DATABASE IF NOT EXISTS openflare;

USE openflare;

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
