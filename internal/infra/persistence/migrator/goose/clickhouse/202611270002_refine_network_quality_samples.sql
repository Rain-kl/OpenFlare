-- +goose Up
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS client_region String DEFAULT '';
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS ip_family String DEFAULT 'unknown';
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS stats_version Int32 DEFAULT 1;
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS rtt_hist String DEFAULT '';
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS prefix_sketch String DEFAULT '';
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS distinct_prefix_count Int32 DEFAULT 0;
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS client_lat Nullable(Float64);
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS client_lon Nullable(Float64);
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS geo_accuracy_km Float64 DEFAULT 0;
ALTER TABLE of_network_quality_samples ADD COLUMN IF NOT EXISTS geo_source String DEFAULT '';

-- +goose Down
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS geo_source;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS geo_accuracy_km;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS client_lon;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS client_lat;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS distinct_prefix_count;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS prefix_sketch;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS rtt_hist;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS stats_version;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS ip_family;
ALTER TABLE of_network_quality_samples DROP COLUMN IF EXISTS client_region;
