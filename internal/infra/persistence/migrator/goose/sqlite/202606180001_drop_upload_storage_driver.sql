-- +goose Up
DROP INDEX IF EXISTS idx_w_uploads_storage_driver_status;
-- SQLite lacks DROP COLUMN IF EXISTS; goose runs this only after initial schema created the column.
ALTER TABLE w_uploads DROP COLUMN storage_driver;

-- +goose Down
ALTER TABLE w_uploads ADD COLUMN storage_driver VARCHAR(50) NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS idx_w_uploads_storage_driver_status ON w_uploads (storage_driver, status);