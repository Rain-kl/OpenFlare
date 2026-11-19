-- +goose Up
ALTER TABLE of_config_versions
    ADD COLUMN IF NOT EXISTS engine VARCHAR(32) NOT NULL DEFAULT 'openresty',
    ADD COLUMN IF NOT EXISTS release_sequence BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payload_json TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payload_checksum VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE of_nodes
    ADD COLUMN IF NOT EXISTS engine VARCHAR(32) NOT NULL DEFAULT 'openresty',
    ADD COLUMN IF NOT EXISTS contract_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_apply_logs
    ADD COLUMN IF NOT EXISTS engine VARCHAR(32) NOT NULL DEFAULT 'openresty',
    ADD COLUMN IF NOT EXISTS release_sequence BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payload_checksum VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_nodes_engine ON of_nodes (engine);
CREATE INDEX IF NOT EXISTS idx_of_config_versions_engine_active
    ON of_config_versions (engine, is_active);

-- +goose Down
DROP INDEX IF EXISTS idx_of_config_versions_engine_active;
DROP INDEX IF EXISTS idx_of_nodes_engine;
ALTER TABLE of_nodes
    DROP COLUMN IF EXISTS contract_version,
    DROP COLUMN IF EXISTS engine;
ALTER TABLE of_apply_logs
    DROP COLUMN IF EXISTS payload_checksum,
    DROP COLUMN IF EXISTS release_sequence,
    DROP COLUMN IF EXISTS engine;
ALTER TABLE of_config_versions
    DROP COLUMN IF EXISTS payload_checksum,
    DROP COLUMN IF EXISTS payload_json,
    DROP COLUMN IF EXISTS release_sequence,
    DROP COLUMN IF EXISTS engine;
