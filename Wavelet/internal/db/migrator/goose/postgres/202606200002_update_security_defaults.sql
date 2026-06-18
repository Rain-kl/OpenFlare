-- +goose Up
UPDATE w_system_configs
SET value = 'false', updated_at = CURRENT_TIMESTAMP
WHERE key = 'registration_enabled' AND value = 'true';

UPDATE w_system_configs
SET value = 'false', updated_at = CURRENT_TIMESTAMP
WHERE key = 'password_register_enabled' AND value = 'true';

UPDATE w_system_configs
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'cap_login_enabled' AND value = 'false';

-- +goose Down
UPDATE w_system_configs
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'registration_enabled' AND value = 'false';

UPDATE w_system_configs
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'password_register_enabled' AND value = 'false';

UPDATE w_system_configs
SET value = 'false', updated_at = CURRENT_TIMESTAMP
WHERE key = 'cap_login_enabled' AND value = 'true';