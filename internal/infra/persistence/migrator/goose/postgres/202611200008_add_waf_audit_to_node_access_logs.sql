-- +goose Up
ALTER TABLE of_node_access_logs ADD COLUMN waf_action VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE of_node_access_logs ADD COLUMN waf_audit TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_node_access_log_waf_action ON of_node_access_logs (logged_at DESC) WHERE waf_action <> '';

-- +goose Down
DROP INDEX idx_node_access_log_waf_action;
ALTER TABLE of_node_access_logs DROP COLUMN waf_audit;
ALTER TABLE of_node_access_logs DROP COLUMN waf_action;
