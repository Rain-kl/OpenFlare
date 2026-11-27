-- +goose Up
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS observed_rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS short_rtt_p50_ms DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS short_rtt_p95_ms DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS short_sample_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS quality_state VARCHAR(16) NOT NULL DEFAULT 'healthy';
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS consecutive_good_windows INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN IF NOT EXISTS explore_priority DOUBLE PRECISION NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS explore_priority;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS consecutive_good_windows;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS quality_state;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS short_sample_count;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS short_rtt_p95_ms;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS short_rtt_p50_ms;
ALTER TABLE of_network_quality_matrix DROP COLUMN IF EXISTS observed_rtt_ms;
