-- +goose Up
CREATE TABLE IF NOT EXISTS of_tls_certificates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    cert_pem TEXT NOT NULL,
    key_pem TEXT NOT NULL,
    not_before DATETIME,
    not_after DATETIME,
    remark TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT 'upload',
    acme_account_id INTEGER NOT NULL DEFAULT 0,
    dns_account_id INTEGER NOT NULL DEFAULT 0,
    key_algorithm TEXT NOT NULL DEFAULT '',
    auto_renew INTEGER NOT NULL DEFAULT 0,
    primary_domain TEXT NOT NULL DEFAULT '',
    other_domains TEXT NOT NULL DEFAULT '',
    disable_cname INTEGER NOT NULL DEFAULT 0,
    skip_dns INTEGER NOT NULL DEFAULT 0,
    dns1 TEXT NOT NULL DEFAULT '',
    dns2 TEXT NOT NULL DEFAULT '',
    apply_status TEXT NOT NULL DEFAULT 'ready',
    apply_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tls_certificates_name ON of_tls_certificates (name);

CREATE TABLE IF NOT EXISTS of_managed_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    cert_id INTEGER,
    enabled INTEGER NOT NULL DEFAULT 1,
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_managed_domains_domain ON of_managed_domains (domain);

CREATE TABLE IF NOT EXISTS of_dns_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    authorization TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS of_acme_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    private_key TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS of_managed_domains;
DROP TABLE IF EXISTS of_tls_certificates;
DROP TABLE IF EXISTS of_dns_accounts;
DROP TABLE IF EXISTS of_acme_accounts;