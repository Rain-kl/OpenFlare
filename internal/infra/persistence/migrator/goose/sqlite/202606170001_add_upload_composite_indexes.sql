-- +goose Up
CREATE INDEX IF NOT EXISTS idx_w_uploads_status_created_at ON w_uploads (status, created_at);
CREATE INDEX IF NOT EXISTS idx_w_uploads_storage_driver_status ON w_uploads (storage_driver, status);
CREATE INDEX IF NOT EXISTS idx_w_uploads_hash_file_size_status ON w_uploads (hash, file_size, status);

-- +goose Down
DROP INDEX IF EXISTS idx_w_uploads_hash_file_size_status;
DROP INDEX IF EXISTS idx_w_uploads_storage_driver_status;
DROP INDEX IF EXISTS idx_w_uploads_status_created_at;