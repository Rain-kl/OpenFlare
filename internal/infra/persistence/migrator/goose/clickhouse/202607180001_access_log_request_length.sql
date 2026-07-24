-- +goose Up
-- L1 access log: request body/header length (接收数据) and optional request duration.
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS request_length UInt64 DEFAULT 0;

ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS request_time_ms UInt32 DEFAULT 0;

-- +goose Down
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS request_time_ms;
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS request_length;
