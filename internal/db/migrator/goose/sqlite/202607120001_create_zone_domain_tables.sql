-- +goose Up
CREATE TABLE IF NOT EXISTS of_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zones_domain ON of_zones (domain);

CREATE TABLE IF NOT EXISTS of_zone_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id INTEGER NOT NULL,
    proxy_route_id INTEGER,
    domain TEXT NOT NULL,
    cert_id INTEGER,
    remark TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zone_domains_domain ON of_zone_domains (domain);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_zone_id ON of_zone_domains (zone_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_proxy_route_id ON of_zone_domains (proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_cert_id ON of_zone_domains (cert_id);

-- +goose Down
DROP TABLE IF EXISTS of_zone_domains;
DROP TABLE IF EXISTS of_zones;
