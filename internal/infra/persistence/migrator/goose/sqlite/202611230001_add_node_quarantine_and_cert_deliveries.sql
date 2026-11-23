-- +goose Up
ALTER TABLE of_nodes ADD COLUMN quarantined_at DATETIME;
ALTER TABLE of_nodes ADD COLUMN quarantine_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_of_nodes_quarantined_at ON of_nodes (quarantined_at);

CREATE TABLE IF NOT EXISTS of_node_cert_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    certificate_id INTEGER NOT NULL,
    cert_sha256 TEXT NOT NULL DEFAULT '',
    release_sequence INTEGER NOT NULL DEFAULT 0,
    delivery_count INTEGER NOT NULL DEFAULT 1,
    first_delivered_at DATETIME NOT NULL,
    last_delivered_at DATETIME NOT NULL
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
ALTER TABLE of_nodes DROP COLUMN quarantine_reason;
ALTER TABLE of_nodes DROP COLUMN quarantined_at;
