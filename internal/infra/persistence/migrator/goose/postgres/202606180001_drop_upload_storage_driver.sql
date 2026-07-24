-- +goose Up
DROP INDEX IF EXISTS idx_w_uploads_storage_driver_status;
ALTER TABLE w_uploads DROP COLUMN IF EXISTS storage_driver;

-- +goose Down
ALTER TABLE w_uploads ADD COLUMN IF NOT EXISTS storage_driver VARCHAR(50) NOT NULL DEFAULT 'local';
CREATE INDEX IF NOT EXISTS idx_w_uploads_storage_driver_status ON w_uploads (storage_driver, status);