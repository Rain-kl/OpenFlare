-- +goose Up
UPDATE of_config_versions
SET is_active = false
WHERE is_active = true
  AND EXISTS (
      SELECT 1 FROM of_config_versions newer
      WHERE newer.engine = of_config_versions.engine
        AND newer.is_active = true
        AND newer.version > of_config_versions.version
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_config_versions_one_active_per_engine
    ON of_config_versions (engine) WHERE is_active = true;

-- +goose Down
DROP INDEX IF EXISTS idx_of_config_versions_one_active_per_engine;
