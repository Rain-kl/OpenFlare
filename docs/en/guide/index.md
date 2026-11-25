# Guide Overview

OpenFlare is a distributed reverse proxy and edge security gateway based on OpenFlare Edge architecture. It provides multi-node cluster orchestration, snapshot-based configuration hot-synchronization, edge WAF, proof-of-work (PoW) challenge defense, intranet tunnels, and Pages static hosting. It can operate as a standalone gateway or as an origin shield behind upstream CDNs.

Documentation in this section is organized into five operational areas, guiding users from initial deployment through daily traffic management, security hardening, and troubleshooting.

---

## Guide Index

### Getting Started
For users deploying OpenFlare or connecting edge nodes for the first time:
- [Quick Start](./quick-start.md): deploy the Server controller with Docker Compose, complete first-time login, obtain credentials, and connect your first edge node.
- [Create Your First Reverse Proxy Rule](./first-site.md): create a baseline HTTP proxy site and verify edge node synchronization.
- [Dashboard Geographic Distribution](./dashboard-map.md): explore the global dashboard, 2D/3D map switching, metric calculation methods, and country request rankings.

### Traffic & Routing
Configure reverse proxy routing, TLS encryption, and edge caching:
- [Create a Reverse Proxy Config](./proxy-config.md): configure upstream origins, domain route bindings, wildcard routing, IP HTTPS upstreams, and access authentication.
- [TLS Certificates & Auto-Renewal](./certificates.md): import existing certificates or automate wildcard issuance and renewal using ACME and Cloudflare DNS API.
- [OpenFlare Edge Performance Policies](./performance-policies.md): configure worker thread counts, upstream connection pools, HTTP/2 concurrency, TinyUFO memory cache, and disk cache limits.
- [Zone Domain Migration](./zone-domain-migration.md): understand automatic migration to the Zone root domain model, backup procedures, and post-upgrade acceptance checks.

### Edge Security
Configure edge firewalls and proof-of-work challenge defenses:
- [WAF Security Protection](./waf-usage.md): build visual DAG workflow rules, configure IP matching, geo-filtering, and OWASP CRS attack protection rule groups.
- [Proof-of-Work (PoW) Protection](./pow-usage.md): configure client-side proof-of-work challenges for routes, manage trigger path filters, and customize challenge pages.
- [WAF Auto IP Group Expressions](./waf-ip-group-expr.md): write Expr-based dynamic metric aggregation and auto-blocking rules with custom HTTP status code filters.

### Apps & Integrations
Extend gateway capabilities with static hosting, intranet tunnels, and monitoring:
- [Pages Static Hosting Usage](./pages-usage.md): deploy pre-built static site archives, configure SPA Fallback and API reverse proxy paths, and manage release rollbacks.
- [Tunnel & Intranet Penetration](./tunnel-usage.md): deploy Relay nodes and OpenFlared clients to securely expose intranet services without public IP addresses.
- [Uptime Kuma Monitoring Sync](./uptime-kuma.md): configure automated differential synchronization with Uptime Kuma to track site availability.
- [SSO Login Configuration](./sso.md): configure standard OIDC identity providers for third-party single sign-on, user binding, and registration controls.

### Operations
Maintenance, diagnostic procedures, and open-source references:
- [Troubleshooting](./troubleshooting.md): resolve issues with controller startup, database connections, Agent registration, OpenFlare Edge configuration application, and edge cache misses.
- [Open Source Acknowledgments](./credits.md): review core open-source projects, derived code provenance, third-party components, and license notices.

---

## Common Scenarios

| Goal | Recommended Guide |
| --- | --- |
| Start controller and connect first node | [Quick Start](./quick-start.md) |
| Test basic HTTP proxy routing | [Create Your First Reverse Proxy Rule](./first-site.md) |
| Configure HTTPS certificates and origin routing | [Create a Reverse Proxy Config](./proxy-config.md) and [TLS Certificates & Auto-Renewal](./certificates.md) |
| Tune worker threads, connection pools, and cache sizes | [OpenFlare Edge Performance Policies](./performance-policies.md) |
| Enable WAF blocking and anti-CC challenge rules | [WAF Security Protection](./waf-usage.md) and [Proof-of-Work (PoW) Protection](./pow-usage.md) |
| Host single-page applications with API proxying | [Pages Static Hosting Usage](./pages-usage.md) |
| Expose intranet services to the public internet | [Tunnel & Intranet Penetration](./tunnel-usage.md) |
| Synchronize site uptime monitoring | [Uptime Kuma Monitoring Sync](./uptime-kuma.md) |
| Configure enterprise OIDC / OAuth2 login | [SSO Login Configuration](./sso.md) |
| Diagnose failed configuration sync or cache misses | [Troubleshooting](./troubleshooting.md) |

---

## Documentation Structure

- [Deployment](../deployment/index.md): choose a deployment method, install components, and upgrade nodes.
- [Reference](../reference/index.md): look up configuration settings, commands, protocols, and benchmarks.
- [Architecture](../design/index.md): explore system architecture, data models, configuration synchronization, and security.
- [Changelog](../changelog/index.md): review release changes and upgrade notes.

---

## Community & Support

- [Telegram Group](https://t.me/OpenFlareCDN): usage discussions, deployment questions, and community feedback.
- [GitHub Issues](https://github.com/Rain-kl/OpenFlare/issues): bug reports and feature requests.
