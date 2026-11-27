-- +goose Up
INSERT INTO of_probe_targets (
    name, ip, ip_family, probe_proto, port, asn, country, region, target_type, anycast_status, enabled, notes, created_at, updated_at
)
VALUES
    ('Cloudflare DNS IPv4 Anchor', '1.1.1.1', 'ipv4', 'icmp', 0, 13335, 'US', '', 'public_anchor', 'confirmed', TRUE, 'Global anycast DNS reachability anchor (isolated from regional generalization)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Google DNS IPv4 Anchor', '8.8.8.8', 'ipv4', 'icmp', 0, 15169, 'US', '', 'public_anchor', 'confirmed', TRUE, 'Global anycast DNS reachability anchor (isolated from regional generalization)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Quad9 DNS IPv4 Anchor', '9.9.9.9', 'ipv4', 'icmp', 0, 19281, 'US', '', 'public_anchor', 'confirmed', TRUE, 'Global anycast DNS reachability anchor (isolated from regional generalization)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Cloudflare DNS IPv6 Anchor', '2606:4700:4700::1111', 'ipv6', 'icmp', 0, 13335, 'US', '', 'public_anchor', 'confirmed', TRUE, 'Global anycast IPv6 reachability anchor (isolated from regional generalization)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (ip, probe_proto, port) DO NOTHING;

-- +goose Down
DELETE FROM of_probe_targets
WHERE (ip = '1.1.1.1' AND probe_proto = 'icmp' AND port = 0)
   OR (ip = '8.8.8.8' AND probe_proto = 'icmp' AND port = 0)
   OR (ip = '9.9.9.9' AND probe_proto = 'icmp' AND port = 0)
   OR (ip = '2606:4700:4700::1111' AND probe_proto = 'icmp' AND port = 0);
