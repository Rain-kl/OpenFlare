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
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    logged_at TIMESTAMPTZ NOT NULL,
    remote_addr VARCHAR(128) NOT NULL DEFAULT '',
    region VARCHAR(128) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL DEFAULT '',
    path VARCHAR(2048) NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_of_node_access_logs_node_id ON of_node_access_logs (node_id);
CREATE INDEX idx_of_node_access_logs_logged_at ON of_node_access_logs (logged_at);
CREATE INDEX idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr);
CREATE INDEX idx_of_node_access_logs_host ON of_node_access_logs (host);
CREATE INDEX idx_of_node_access_logs_status_code ON of_node_access_logs (status_code);
CREATE INDEX idx_of_node_access_logs_node_id_logged_at ON of_node_access_logs (node_id, logged_at);