-- +goose Up
ALTER TABLE of_attack_groups ADD COLUMN rule_overrides TEXT NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE of_attack_groups DROP COLUMN rule_overrides;
