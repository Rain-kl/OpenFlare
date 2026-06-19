-- +goose Up
CREATE TABLE of_apply_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    version TEXT NOT NULL,
    result TEXT NOT NULL,
    message TEXT,
    checksum TEXT NOT NULL DEFAULT '',
    main_config_checksum TEXT NOT NULL DEFAULT '',
    route_config_checksum TEXT NOT NULL DEFAULT '',
    support_file_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_of_apply_logs_node_id ON of_apply_logs(node_id);
CREATE INDEX idx_of_apply_logs_created_at ON of_apply_logs(created_at);

-- +goose Down
DROP TABLE IF EXISTS of_apply_logs;