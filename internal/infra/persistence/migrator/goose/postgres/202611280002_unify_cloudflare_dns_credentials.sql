-- +goose Up
INSERT INTO of_dns_accounts (name, type, "authorization", created_at, updated_at)
SELECT 'Cloudflare (migrated connection)', 'cloudflare', "authorization", CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM of_cf_connections
WHERE id = 1 AND source = 'standalone' AND "authorization" <> '';

UPDATE of_cf_connections
SET source = 'dns_account',
    dns_account_id = (
        SELECT id FROM of_dns_accounts
        WHERE type = 'cloudflare' AND name = 'Cloudflare (migrated connection)'
          AND "authorization" = of_cf_connections."authorization"
        ORDER BY id DESC LIMIT 1
    ),
    "authorization" = ''
WHERE id = 1 AND source = 'standalone' AND "authorization" <> '';

UPDATE of_cf_pointing_members
SET dns_account_id = (SELECT dns_account_id FROM of_cf_connections WHERE id = 1)
WHERE cf_record_id <> '';

-- +goose Down
-- The linked credential is compatible with older versions and remains in the DNS account.
SELECT 1;
