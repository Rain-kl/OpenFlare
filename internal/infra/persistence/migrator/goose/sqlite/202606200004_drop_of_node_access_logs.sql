-- +goose Up
DROP INDEX IF EXISTS idx_of_node_access_logs_node_id_logged_at;
DROP INDEX IF EXISTS idx_of_node_access_logs_status_code;
DROP INDEX IF EXISTS idx_of_node_access_logs_host;
DROP INDEX IF EXISTS idx_of_node_access_logs_remote_addr;
DROP INDEX IF EXISTS idx_of_node_access_logs_logged_at;
DROP INDEX IF EXISTS idx_of_node_access_logs_node_id;
DROP TABLE IF EXISTS of_node_access_logs;

-- +goose Down
CREATE TABLE of_node_access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    logged_at DATETIME NOT NULL,
    remote_addr TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    host TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_access_logs_node_id ON of_node_access_logs (node_id);
CREATE INDEX idx_of_node_access_logs_logged_at ON of_node_access_logs (logged_at);
CREATE INDEX idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr);
CREATE INDEX idx_of_node_access_logs_host ON of_node_access_logs (host);
CREATE INDEX idx_of_node_access_logs_status_code ON of_node_access_logs (status_code);
CREATE INDEX idx_of_node_access_logs_node_id_logged_at ON of_node_access_logs (node_id, logged_at);