-- +goose Up
CREATE TABLE IF NOT EXISTS w_upload_stats (
    dimension VARCHAR(32) NOT NULL,
    stat_key VARCHAR(64) NOT NULL DEFAULT '',
    file_count BIGINT NOT NULL DEFAULT 0,
    file_size BIGINT NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (dimension, stat_key)
);

-- +goose Down
DROP TABLE IF EXISTS w_upload_stats;