-- +goose Up
ALTER TABLE of_pages_deployments
    ADD COLUMN upload_id INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_upload_id ON of_pages_deployments (upload_id);

-- +goose Down
DROP INDEX IF EXISTS idx_of_pages_deployments_upload_id;

-- SQLite cannot drop columns without table rebuild; keep upload_id on rollback.