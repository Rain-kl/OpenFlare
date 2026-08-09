-- +goose Up
-- 节点访问日志：普通表（同 PG 语义，索引名保持一致）。
CREATE TABLE IF NOT EXISTS of_node_access_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id         TEXT NOT NULL DEFAULT '',
    logged_at       DATETIME NOT NULL,
    remote_addr     TEXT NOT NULL DEFAULT '',
    region          TEXT NOT NULL DEFAULT '',
    host            TEXT NOT NULL DEFAULT '',
    path            TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    TEXT NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      INTEGER NOT NULL DEFAULT 0,
    request_length  INTEGER NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_node_id ON of_node_access_logs (node_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host ON of_node_access_logs (host, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_status_code ON of_node_access_logs (status_code, logged_at DESC);

CREATE TABLE IF NOT EXISTS w_user_access_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL DEFAULT 0,
    path       TEXT NOT NULL DEFAULT '',
    method     TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    headers    TEXT NOT NULL DEFAULT '',
    status     INTEGER NOT NULL DEFAULT 0,
    latency    INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_user_id ON w_user_access_logs (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS of_node_metric_snapshots (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id             TEXT NOT NULL DEFAULT '',
    captured_at         DATETIME NOT NULL,
    cpu_usage_percent   REAL NOT NULL DEFAULT 0,
    memory_used_bytes   INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes  INTEGER NOT NULL DEFAULT 0,
    storage_used_bytes  INTEGER NOT NULL DEFAULT 0,
    storage_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes     INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes    INTEGER NOT NULL DEFAULT 0,
    network_rx_bytes    INTEGER NOT NULL DEFAULT 0,
    network_tx_bytes    INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node ON of_node_metric_snapshots (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_edge_health (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id     TEXT NOT NULL DEFAULT '',
    captured_at DATETIME NOT NULL,
    status      TEXT NOT NULL DEFAULT '',
    connections INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id           TEXT NOT NULL DEFAULT '',
    captured_at       DATETIME NOT NULL,
    frps_connections  INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count  INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies      TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id                TEXT NOT NULL DEFAULT '',
    captured_at            DATETIME NOT NULL,
    tunnel_status          TEXT NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

-- +goose Down
DROP TABLE IF EXISTS of_node_obs_frpc;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_edge_health;
DROP TABLE IF EXISTS of_node_metric_snapshots;
DROP TABLE IF EXISTS w_user_access_logs;
DROP TABLE IF EXISTS of_node_access_logs;
