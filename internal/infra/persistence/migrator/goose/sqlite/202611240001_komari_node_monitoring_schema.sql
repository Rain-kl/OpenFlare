-- +goose Up
-- Enrich node system profile with Komari hardware/OS inventory fields.
ALTER TABLE of_node_system_profiles ADD COLUMN virtualization TEXT NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN gpu_name TEXT NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN swap_total_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_node_system_profiles ADD COLUMN cpu_physical_cores INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_node_system_profiles ADD COLUMN ipv4 TEXT NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN ipv6 TEXT NOT NULL DEFAULT '';
ALTER TABLE of_node_system_profiles ADD COLUMN probe_version TEXT NOT NULL DEFAULT '';

-- Recreate node metric time-series table adopting Komari Record schema (no historical data migration).
DROP TABLE IF EXISTS of_node_metric_snapshots;
CREATE TABLE of_node_metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    cpu REAL NOT NULL DEFAULT 0,
    gpu REAL NOT NULL DEFAULT 0,
    ram INTEGER NOT NULL DEFAULT 0,
    ram_total INTEGER NOT NULL DEFAULT 0,
    swap INTEGER NOT NULL DEFAULT 0,
    swap_total INTEGER NOT NULL DEFAULT 0,
    load REAL NOT NULL DEFAULT 0,
    load5 REAL NOT NULL DEFAULT 0,
    load15 REAL NOT NULL DEFAULT 0,
    temp REAL NOT NULL DEFAULT 0,
    disk INTEGER NOT NULL DEFAULT 0,
    disk_total INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes INTEGER NOT NULL DEFAULT 0,
    net_in INTEGER NOT NULL DEFAULT 0,
    net_out INTEGER NOT NULL DEFAULT 0,
    net_total_up INTEGER NOT NULL DEFAULT 0,
    net_total_down INTEGER NOT NULL DEFAULT 0,
    process INTEGER NOT NULL DEFAULT 0,
    connections INTEGER NOT NULL DEFAULT 0,
    connections_udp INTEGER NOT NULL DEFAULT 0,
    uptime INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node_captured ON of_node_metric_snapshots (node_id, captured_at DESC);

-- Latency monitoring time-series table (Komari PingRecord schema).
CREATE TABLE IF NOT EXISTS of_node_ping_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    task_id INTEGER NOT NULL,
    recorded_at DATETIME NOT NULL,
    value INTEGER NOT NULL DEFAULT -1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_ping_records_node_task_time ON of_node_ping_records (node_id, task_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_ping_records_task_time ON of_node_ping_records (task_id, recorded_at DESC);

-- Latency monitoring task definitions (Komari PingTask schema).
CREATE TABLE IF NOT EXISTS of_ping_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    weight INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    clients TEXT NOT NULL DEFAULT '[]',
    all_clients INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL DEFAULT 'icmp',
    target TEXT NOT NULL,
    interval INTEGER NOT NULL DEFAULT 60,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_ping_tasks_weight ON of_ping_tasks (weight ASC, id ASC);
CREATE INDEX IF NOT EXISTS idx_of_ping_tasks_name ON of_ping_tasks (name);

-- Remote command execution tasks and per-node results (Komari Task / TaskResult schema).
CREATE TABLE IF NOT EXISTS of_exec_tasks (
    task_id TEXT PRIMARY KEY,
    clients TEXT NOT NULL DEFAULT '[]',
    command TEXT NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_exec_tasks_created_at ON of_exec_tasks (created_at DESC);

CREATE TABLE IF NOT EXISTS of_exec_task_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    client TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    exit_code INTEGER NULL,
    finished_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    captured_at DATETIME NOT NULL,
    cpu_usage_percent REAL NOT NULL DEFAULT 0,
    memory_used_bytes INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes INTEGER NOT NULL DEFAULT 0,
    storage_used_bytes INTEGER NOT NULL DEFAULT 0,
    storage_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes INTEGER NOT NULL DEFAULT 0,
    network_rx_bytes INTEGER NOT NULL DEFAULT 0,
    network_tx_bytes INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_metric_snapshots_node_captured ON of_node_metric_snapshots (node_id, captured_at DESC);
