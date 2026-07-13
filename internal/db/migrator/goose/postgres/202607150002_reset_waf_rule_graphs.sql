-- +goose Up
UPDATE of_waf_rule_groups
SET graph = '{"schema_version":1,"nodes":[{"id":"start","type":"start","position":{"x":0,"y":0},"config":{}},{"id":"allow","type":"allow","position":{"x":320,"y":0},"config":{}}],"edges":[{"id":"start-allow","source":"start","source_handle":"next","target":"allow"}]}',
    revision = 1;

WITH ordered AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY proxy_route_id ORDER BY id) - 1 AS new_sequence
    FROM of_waf_rule_group_bindings
)
UPDATE of_waf_rule_group_bindings AS binding
SET sequence = ordered.new_sequence
FROM ordered
WHERE binding.id = ordered.id;

-- +goose Down
UPDATE of_waf_rule_group_bindings SET sequence = 0;
UPDATE of_waf_rule_groups SET revision = 1;
