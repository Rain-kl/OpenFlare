-- +goose Up
DELETE FROM of_options
WHERE key IN (
    'GitHubOAuthEnabled',
    'GitHubClientId',
    'GitHubClientSecret',
    'WeChatAuthEnabled',
    'WeChatServerAddress',
    'WeChatServerToken',
    'WeChatAccountQRCodeImageURL'
);

-- +goose Down
INSERT INTO of_options (key, value) VALUES
    ('GitHubOAuthEnabled', 'false'),
    ('GitHubClientId', ''),
    ('GitHubClientSecret', ''),
    ('WeChatAuthEnabled', 'false'),
    ('WeChatServerAddress', ''),
    ('WeChatServerToken', ''),
    ('WeChatAccountQRCodeImageURL', '')
ON CONFLICT (key) DO NOTHING;
