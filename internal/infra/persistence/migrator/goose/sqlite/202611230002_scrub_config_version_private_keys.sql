-- +goose Up
UPDATE of_config_versions
SET support_files_json = '[]', payload_checksum = ''
WHERE support_files_json LIKE '%PRIVATE KEY%';

UPDATE of_edge_snapshot_state
SET source_revision = source_revision + 1
WHERE id = 1;

-- +goose Down
SELECT 1;
