-- +goose Up
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS waf_action LowCardinality(String) DEFAULT '',
    ADD COLUMN IF NOT EXISTS waf_audit String DEFAULT '';

-- +goose Down
ALTER TABLE of_node_access_logs DROP COLUMN IF EXISTS waf_action, DROP COLUMN IF EXISTS waf_audit;
