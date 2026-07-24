-- +goose Up
CREATE TABLE of_apply_logs (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    version VARCHAR(32) NOT NULL,
    result VARCHAR(32) NOT NULL,
    message TEXT,
    checksum VARCHAR(64) NOT NULL DEFAULT '',
    main_config_checksum VARCHAR(64) NOT NULL DEFAULT '',
    route_config_checksum VARCHAR(64) NOT NULL DEFAULT '',
    support_file_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_of_apply_logs_node_id ON of_apply_logs(node_id);
CREATE INDEX idx_of_apply_logs_created_at ON of_apply_logs(created_at);

-- +goose Down
DROP TABLE IF EXISTS of_apply_logs;