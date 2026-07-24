-- +goose Up
UPDATE of_waf_rule_groups
SET graph = '{"schema_version":1,"nodes":[{"id":"start","type":"start","position":{"x":0,"y":0},"config":{}},{"id":"allow","type":"allow","position":{"x":320,"y":0},"config":{}}],"edges":[{"id":"start-allow","source":"start","source_handle":"next","target":"allow"}]}',
    revision = 1;

UPDATE of_waf_rule_group_bindings
SET sequence = (
    SELECT COUNT(*)
    FROM of_waf_rule_group_bindings AS preceding
    WHERE preceding.proxy_route_id = of_waf_rule_group_bindings.proxy_route_id
      AND preceding.id < of_waf_rule_group_bindings.id
);

-- +goose Down
UPDATE of_waf_rule_group_bindings SET sequence = 0;
UPDATE of_waf_rule_groups SET revision = 1;
