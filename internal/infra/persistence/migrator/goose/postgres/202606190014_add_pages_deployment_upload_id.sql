-- +goose Up
ALTER TABLE of_pages_deployments
    ADD COLUMN IF NOT EXISTS upload_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_upload_id ON of_pages_deployments (upload_id);

ALTER TABLE of_pages_deployments
    ALTER COLUMN artifact_path SET DEFAULT '';

-- +goose Down
DROP INDEX IF EXISTS idx_of_pages_deployments_upload_id;

ALTER TABLE of_pages_deployments
    DROP COLUMN IF EXISTS upload_id;