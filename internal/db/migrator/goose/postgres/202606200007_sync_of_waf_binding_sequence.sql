-- +goose Up
SELECT setval(
    pg_get_serial_sequence('of_waf_rule_group_bindings', 'id'),
    COALESCE((SELECT MAX(id) FROM of_waf_rule_group_bindings), 0)
);

-- +goose Down
SELECT 1;