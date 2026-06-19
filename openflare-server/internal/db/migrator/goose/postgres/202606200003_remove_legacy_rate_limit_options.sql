-- +goose Up
DELETE FROM of_options
WHERE key IN (
    'GlobalApiRateLimitNum',
    'GlobalApiRateLimitDuration',
    'GlobalWebRateLimitNum',
    'GlobalWebRateLimitDuration',
    'CriticalRateLimitNum',
    'CriticalRateLimitDuration'
);

-- +goose Down
INSERT INTO of_options (key, value) VALUES
    ('GlobalApiRateLimitNum', '300'),
    ('GlobalApiRateLimitDuration', '180'),
    ('GlobalWebRateLimitNum', '300'),
    ('GlobalWebRateLimitDuration', '180'),
    ('CriticalRateLimitNum', '100'),
    ('CriticalRateLimitDuration', '1200')
ON CONFLICT (key) DO NOTHING;