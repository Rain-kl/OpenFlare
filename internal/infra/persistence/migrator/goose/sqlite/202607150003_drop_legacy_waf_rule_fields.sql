-- +goose Up
ALTER TABLE of_waf_rule_groups DROP COLUMN block_status_code;
ALTER TABLE of_waf_rule_groups DROP COLUMN block_response_body;
ALTER TABLE of_waf_rule_groups DROP COLUMN ip_whitelist;
ALTER TABLE of_waf_rule_groups DROP COLUMN ip_blacklist;
ALTER TABLE of_waf_rule_groups DROP COLUMN ip_whitelist_groups;
ALTER TABLE of_waf_rule_groups DROP COLUMN ip_blacklist_groups;
ALTER TABLE of_waf_rule_groups DROP COLUMN country_whitelist;
ALTER TABLE of_waf_rule_groups DROP COLUMN country_blacklist;
ALTER TABLE of_waf_rule_groups DROP COLUMN region_whitelist;
ALTER TABLE of_waf_rule_groups DROP COLUMN region_blacklist;
ALTER TABLE of_waf_rule_groups DROP COLUMN pow_enabled;
ALTER TABLE of_waf_rule_groups DROP COLUMN pow_config;

-- +goose Down
ALTER TABLE of_waf_rule_groups ADD COLUMN block_status_code INTEGER NOT NULL DEFAULT 418;
ALTER TABLE of_waf_rule_groups ADD COLUMN block_response_body TEXT NOT NULL DEFAULT '';
ALTER TABLE of_waf_rule_groups ADD COLUMN ip_whitelist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN ip_blacklist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN ip_whitelist_groups TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN ip_blacklist_groups TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN country_whitelist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN country_blacklist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN region_whitelist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN region_blacklist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_waf_rule_groups ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE of_waf_rule_groups ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}';
