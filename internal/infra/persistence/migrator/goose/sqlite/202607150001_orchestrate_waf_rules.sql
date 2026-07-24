-- +goose Up
ALTER TABLE of_waf_rule_groups
    ADD COLUMN graph TEXT NOT NULL DEFAULT '';
ALTER TABLE of_waf_rule_groups ADD COLUMN revision INTEGER NOT NULL DEFAULT 1;
ALTER TABLE of_waf_rule_group_bindings ADD COLUMN sequence INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE of_waf_rule_group_bindings DROP COLUMN sequence;
ALTER TABLE of_waf_rule_groups DROP COLUMN revision;
ALTER TABLE of_waf_rule_groups DROP COLUMN graph;
