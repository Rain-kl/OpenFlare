-- +goose Up
INSERT INTO of_performance_policies (preset_key, name, settings_json)
VALUES ('default', 'Pingora', '{"threads_auto":true}')
ON CONFLICT (preset_key) DO NOTHING;

-- +goose Down
UPDATE of_nodes SET performance_policy_id = NULL WHERE performance_policy_id IN (
    SELECT id FROM of_performance_policies WHERE preset_key = 'default'
);
DELETE FROM of_performance_policies WHERE preset_key = 'default';
