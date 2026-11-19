-- +goose Up
ALTER TABLE of_config_versions ADD COLUMN engine TEXT NOT NULL DEFAULT 'openresty';
ALTER TABLE of_config_versions ADD COLUMN release_sequence INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_config_versions ADD COLUMN payload_json TEXT NOT NULL DEFAULT '';
ALTER TABLE of_config_versions ADD COLUMN payload_checksum TEXT NOT NULL DEFAULT '';
ALTER TABLE of_nodes ADD COLUMN engine TEXT NOT NULL DEFAULT 'openresty';
ALTER TABLE of_nodes ADD COLUMN contract_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_apply_logs ADD COLUMN engine TEXT NOT NULL DEFAULT 'openresty';
ALTER TABLE of_apply_logs ADD COLUMN release_sequence INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_apply_logs ADD COLUMN payload_checksum TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_nodes_engine ON of_nodes (engine);
CREATE INDEX IF NOT EXISTS idx_of_config_versions_engine_active
    ON of_config_versions (engine, is_active);

-- +goose Down
DROP INDEX IF EXISTS idx_of_config_versions_engine_active;
DROP INDEX IF EXISTS idx_of_nodes_engine;
ALTER TABLE of_nodes DROP COLUMN contract_version;
ALTER TABLE of_nodes DROP COLUMN engine;
ALTER TABLE of_apply_logs DROP COLUMN payload_checksum;
ALTER TABLE of_apply_logs DROP COLUMN release_sequence;
ALTER TABLE of_apply_logs DROP COLUMN engine;
ALTER TABLE of_config_versions DROP COLUMN payload_checksum;
ALTER TABLE of_config_versions DROP COLUMN payload_json;
ALTER TABLE of_config_versions DROP COLUMN release_sequence;
ALTER TABLE of_config_versions DROP COLUMN engine;
