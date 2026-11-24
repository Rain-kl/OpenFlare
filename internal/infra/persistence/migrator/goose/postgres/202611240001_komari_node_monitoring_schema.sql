-- +goose Up
-- Enrich node system profile with Komari hardware/OS inventory fields.
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS virtualization VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS gpu_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS swap_total_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS cpu_physical_cores INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS ipv4 VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS ipv6 VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN IF NOT EXISTS probe_version VARCHAR(64) NOT NULL DEFAULT '';

-- Recreate node metric time-series table adopting Komari Record schema (no historical data migration).
DROP TABLE IF EXISTS of_node_metric_snapshots;
CREATE TABLE of_node_metric_snapshots (
    id BIGINT PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    cpu DOUBLE PRECISION NOT NULL DEFAULT 0,
    gpu DOUBLE PRECISION NOT NULL DEFAULT 0,
    ram BIGINT NOT NULL DEFAULT 0,
    ram_total BIGINT NOT NULL DEFAULT 0,
    swap BIGINT NOT NULL DEFAULT 0,
    swap_total BIGINT NOT NULL DEFAULT 0,
    load DOUBLE PRECISION NOT NULL DEFAULT 0,
    load5 DOUBLE PRECISION NOT NULL DEFAULT 0,
    load15 DOUBLE PRECISION NOT NULL DEFAULT 0,
    temp DOUBLE PRECISION NOT NULL DEFAULT 0,
    disk BIGINT NOT NULL DEFAULT 0,
    disk_total BIGINT NOT NULL DEFAULT 0,
    disk_read_bytes BIGINT NOT NULL DEFAULT 0,
    disk_write_bytes BIGINT NOT NULL DEFAULT 0,
    net_in BIGINT NOT NULL DEFAULT 0,
    net_out BIGINT NOT NULL DEFAULT 0,
    net_total_up BIGINT NOT NULL DEFAULT 0,
    net_total_down BIGINT NOT NULL DEFAULT 0,
    process INTEGER NOT NULL DEFAULT 0,
    connections INTEGER NOT NULL DEFAULT 0,
    connections_udp INTEGER NOT NULL DEFAULT 0,
    uptime BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node_captured ON of_node_metric_snapshots (node_id, captured_at DESC);

-- Latency monitoring time-series table (Komari PingRecord schema).
CREATE TABLE IF NOT EXISTS of_node_ping_records (
    id BIGINT PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    task_id BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    value INTEGER NOT NULL DEFAULT -1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_of_node_ping_records_node_task_time ON of_node_ping_records (node_id, task_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_ping_records_task_time ON of_node_ping_records (task_id, recorded_at DESC);

-- Latency monitoring task definitions (Komari PingTask schema).
CREATE TABLE IF NOT EXISTS of_ping_tasks (
    id BIGSERIAL PRIMARY KEY,
    weight INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(255) NOT NULL,
    clients TEXT NOT NULL DEFAULT '[]',
    all_clients BOOLEAN NOT NULL DEFAULT FALSE,
    type VARCHAR(16) NOT NULL DEFAULT 'icmp',
    target VARCHAR(255) NOT NULL,
    interval INTEGER NOT NULL DEFAULT 60,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_of_ping_tasks_weight ON of_ping_tasks (weight ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_of_ping_tasks_name ON of_ping_tasks (name);

-- Remote command execution tasks and per-node results (Komari Task / TaskResult schema).
CREATE TABLE IF NOT EXISTS of_exec_tasks (
    task_id VARCHAR(64) PRIMARY KEY,
    clients TEXT NOT NULL DEFAULT '[]',
    command TEXT NOT NULL,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_of_exec_tasks_created_at ON of_exec_tasks (created_at DESC);

CREATE TABLE IF NOT EXISTS of_exec_task_results (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    client VARCHAR(64) NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    exit_code INTEGER NULL,
    finished_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_exec_task_results_task_client ON of_exec_task_results (task_id, client);
CREATE INDEX IF NOT EXISTS idx_of_exec_task_results_client ON of_exec_task_results (client, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_exec_task_results_created_at ON of_exec_task_results (created_at);

-- +goose Down
DROP TABLE IF EXISTS of_exec_task_results;
DROP TABLE IF EXISTS of_exec_tasks;
DROP TABLE IF EXISTS of_ping_tasks;
DROP TABLE IF EXISTS of_node_ping_records;
DROP TABLE IF EXISTS of_node_metric_snapshots;
CREATE TABLE of_node_metric_snapshots (
    id BIGINT PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    cpu_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes BIGINT NOT NULL DEFAULT 0,
    storage_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_read_bytes BIGINT NOT NULL DEFAULT 0,
    disk_write_bytes BIGINT NOT NULL DEFAULT 0,
    network_rx_bytes BIGINT NOT NULL DEFAULT 0,
    network_tx_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node_captured ON of_node_metric_snapshots (node_id, captured_at DESC);
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS probe_version;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS ipv6;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS ipv4;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS cpu_physical_cores;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS swap_total_bytes;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS gpu_name;
ALTER TABLE of_node_system_profiles DROP COLUMN IF EXISTS virtualization;
