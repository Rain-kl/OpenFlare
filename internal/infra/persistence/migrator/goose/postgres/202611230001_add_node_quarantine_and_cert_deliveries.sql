-- +goose Up
ALTER TABLE of_nodes
    ADD COLUMN IF NOT EXISTS quarantined_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS quarantine_reason VARCHAR(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_of_nodes_quarantined_at ON of_nodes (quarantined_at);

CREATE TABLE IF NOT EXISTS of_node_cert_deliveries (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    certificate_id BIGINT NOT NULL,
    cert_sha256 VARCHAR(64) NOT NULL DEFAULT '',
    release_sequence BIGINT NOT NULL DEFAULT 0,
    delivery_count BIGINT NOT NULL DEFAULT 1,
    first_delivered_at TIMESTAMPTZ NOT NULL,
    last_delivered_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_node_cert_deliveries_unique
    ON of_node_cert_deliveries (node_id, certificate_id, cert_sha256);
CREATE INDEX IF NOT EXISTS idx_of_node_cert_deliveries_node_id
    ON of_node_cert_deliveries (node_id, last_delivered_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_cert_deliveries_cert_id
    ON of_node_cert_deliveries (certificate_id, last_delivered_at DESC);

-- +goose Down
DROP TABLE IF EXISTS of_node_cert_deliveries;
DROP INDEX IF EXISTS idx_of_nodes_quarantined_at;
ALTER TABLE of_nodes
    DROP COLUMN IF EXISTS quarantine_reason,
    DROP COLUMN IF EXISTS quarantined_at;
