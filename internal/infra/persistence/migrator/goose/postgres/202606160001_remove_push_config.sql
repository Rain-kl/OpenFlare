-- +goose Up
DELETE FROM w_system_configs WHERE key = 'push_config';
DELETE FROM w_push_events WHERE event_key = 'admin_login';
DELETE FROM w_system_configs WHERE key = 'push_global_token';

-- +goose Down
