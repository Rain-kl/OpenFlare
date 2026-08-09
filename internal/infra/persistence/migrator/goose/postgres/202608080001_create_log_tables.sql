-- +goose Up
-- 节点访问日志：按月 RANGE 分区，复合主键 (id, logged_at) 满足分区键进唯一索引要求。
CREATE TABLE IF NOT EXISTS of_node_access_logs (
    id              BIGINT NOT NULL,
    node_id         VARCHAR(64) NOT NULL DEFAULT '',
    logged_at       TIMESTAMPTZ NOT NULL,
    remote_addr     VARCHAR(128) NOT NULL DEFAULT '',
    region          VARCHAR(128) NOT NULL DEFAULT '',
    host            VARCHAR(255) NOT NULL DEFAULT '',
    path            VARCHAR(2048) NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    VARCHAR(64) NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      BIGINT NOT NULL DEFAULT 0,
    request_length  BIGINT NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, logged_at)
) PARTITION BY RANGE (logged_at);

CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_node_id ON of_node_access_logs (node_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host ON of_node_access_logs (host, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_status_code ON of_node_access_logs (status_code, logged_at DESC);

-- 用户访问日志：按月分区。
CREATE TABLE IF NOT EXISTS w_user_access_logs (
    id          BIGINT NOT NULL,
    user_id     BIGINT NOT NULL DEFAULT 0,
    path        VARCHAR(2048) NOT NULL DEFAULT '',
    method      VARCHAR(16) NOT NULL DEFAULT '',
    ip          VARCHAR(128) NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    headers     TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    latency     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_user_id ON w_user_access_logs (user_id, created_at DESC);

-- 可观测 4 表：普通表 + (node_id, captured_at DESC) 索引。
CREATE TABLE IF NOT EXISTS of_node_metric_snapshots (
    id                 BIGINT NOT NULL PRIMARY KEY,
    node_id            VARCHAR(64) NOT NULL DEFAULT '',
    captured_at        TIMESTAMPTZ NOT NULL,
    cpu_usage_percent  DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes  BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes BIGINT NOT NULL DEFAULT 0,
    storage_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_read_bytes    BIGINT NOT NULL DEFAULT 0,
    disk_write_bytes   BIGINT NOT NULL DEFAULT 0,
    network_rx_bytes   BIGINT NOT NULL DEFAULT 0,
    network_tx_bytes   BIGINT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node ON of_node_metric_snapshots (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_edge_health (
    id          BIGINT NOT NULL PRIMARY KEY,
    node_id     VARCHAR(64) NOT NULL DEFAULT '',
    captured_at TIMESTAMPTZ NOT NULL,
    status      VARCHAR(64) NOT NULL DEFAULT '',
    connections BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frps (
    id                BIGINT NOT NULL PRIMARY KEY,
    node_id           VARCHAR(64) NOT NULL DEFAULT '',
    captured_at       TIMESTAMPTZ NOT NULL,
    frps_connections  INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count  INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies      TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS of_node_obs_frpc (
    id                     BIGINT NOT NULL PRIMARY KEY,
    node_id                VARCHAR(64) NOT NULL DEFAULT '',
    captured_at            TIMESTAMPTZ NOT NULL,
    tunnel_status          VARCHAR(16) NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

-- 分区预建：创建当月及未来 2 个月分区（共 3 个月）。
-- +goose StatementBegin
DO $$
DECLARE
    d date;
BEGIN
    FOR d IN SELECT generate_series(date_trunc('month', now())::date, (date_trunc('month', now()) + interval '2 months')::date, interval '1 month')::date
    LOOP
        EXECUTE format('CREATE TABLE IF NOT EXISTS of_node_access_logs_%s PARTITION OF of_node_access_logs FOR VALUES FROM (%L) TO (%L)',
            to_char(d, 'YYYYMM'), d, d + interval '1 month');
        EXECUTE format('CREATE TABLE IF NOT EXISTS w_user_access_logs_%s PARTITION OF w_user_access_logs FOR VALUES FROM (%L) TO (%L)',
            to_char(d, 'YYYYMM'), d, d + interval '1 month');
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS w_user_access_logs;
DROP TABLE IF EXISTS of_node_access_logs;
DROP TABLE IF EXISTS of_node_metric_snapshots;
DROP TABLE IF EXISTS of_node_edge_health;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_frpc;
