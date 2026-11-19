-- +goose Up
ALTER TABLE of_config_versions ADD COLUMN source_version VARCHAR(32) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_of_config_versions_source_version ON of_config_versions (source_version);

UPDATE of_config_versions SET is_active = false WHERE engine = 'pingora';
ALTER TABLE of_config_versions DROP COLUMN payload_json;

-- +goose Down
ALTER TABLE of_config_versions ADD COLUMN payload_json TEXT NOT NULL DEFAULT '';
DROP INDEX IF EXISTS idx_of_config_versions_source_version;
ALTER TABLE of_config_versions DROP COLUMN source_version;
