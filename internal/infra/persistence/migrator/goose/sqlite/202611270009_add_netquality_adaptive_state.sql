-- +goose Up
ALTER TABLE of_network_quality_matrix ADD COLUMN observed_rtt_ms REAL NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN short_rtt_p50_ms REAL NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN short_rtt_p95_ms REAL NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN short_sample_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN quality_state TEXT NOT NULL DEFAULT 'healthy';
ALTER TABLE of_network_quality_matrix ADD COLUMN consecutive_good_windows INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_network_quality_matrix ADD COLUMN explore_priority REAL NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE of_network_quality_matrix DROP COLUMN explore_priority;
ALTER TABLE of_network_quality_matrix DROP COLUMN consecutive_good_windows;
ALTER TABLE of_network_quality_matrix DROP COLUMN quality_state;
ALTER TABLE of_network_quality_matrix DROP COLUMN short_sample_count;
ALTER TABLE of_network_quality_matrix DROP COLUMN short_rtt_p95_ms;
ALTER TABLE of_network_quality_matrix DROP COLUMN short_rtt_p50_ms;
ALTER TABLE of_network_quality_matrix DROP COLUMN observed_rtt_ms;
