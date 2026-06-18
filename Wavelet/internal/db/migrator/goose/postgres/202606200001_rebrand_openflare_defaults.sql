-- +goose Up
UPDATE w_system_configs
SET value = 'OpenFlare', updated_at = CURRENT_TIMESTAMP
WHERE key = 'site_name' AND value = 'Wavelet';

UPDATE w_system_configs
SET value = 'Rain-kl/OpenFlare', updated_at = CURRENT_TIMESTAMP
WHERE key = 'update_upstream_repository' AND value = 'Rain-kl/Wavelet';

UPDATE w_templates
SET
    subject = REPLACE(subject, 'Wavelet', 'OpenFlare'),
    content = REPLACE(content, 'Wavelet', 'OpenFlare'),
    updated_at = CURRENT_TIMESTAMP
WHERE key IN ('login_email', 'register_email');

-- +goose Down
UPDATE w_system_configs
SET value = 'Wavelet', updated_at = CURRENT_TIMESTAMP
WHERE key = 'site_name' AND value = 'OpenFlare';

UPDATE w_system_configs
SET value = 'Rain-kl/Wavelet', updated_at = CURRENT_TIMESTAMP
WHERE key = 'update_upstream_repository' AND value = 'Rain-kl/OpenFlare';

UPDATE w_templates
SET
    subject = REPLACE(subject, 'OpenFlare', 'Wavelet'),
    content = REPLACE(content, 'OpenFlare', 'Wavelet'),
    updated_at = CURRENT_TIMESTAMP
WHERE key IN ('login_email', 'register_email');