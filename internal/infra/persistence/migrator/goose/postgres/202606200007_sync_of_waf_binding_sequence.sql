-- +goose Up
SELECT setval(
    pg_get_serial_sequence('of_waf_rule_group_bindings', 'id'),
    GREATEST(COALESCE((SELECT MAX(id) FROM of_waf_rule_group_bindings), 0), 1),
    COALESCE((SELECT MAX(id) FROM of_waf_rule_group_bindings), 0) > 0
);

-- +goose Down
SELECT 1;