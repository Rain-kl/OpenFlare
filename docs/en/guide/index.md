# Guide

You will learn: how the OpenFlare docs are organized, which pages to read on first run, and where to start for deployment, usage, and troubleshooting.

OpenFlare is a self-hosted OpenResty control plane. It brings reverse proxy website configs, config version release, Agent node sync, TLS certificates, and basic observability into one admin panel — suitable for a single team or organization managing multiple proxy nodes.

## Recommended Reading Path

If you're new to OpenFlare, read in this order:

1. [Quick Start](./quick-start.md): start the Server with Docker Compose, log in to the admin panel, and connect your first Agent.
2. [Publish First Configuration](./first-site.md): quickly create a basic HTTP reverse proxy site rule and verify the node applied it.
3. [Create a Reverse Proxy Config](./proxy-config.md): step by step, from certificate import and application to HTTPS, upstream origins, and edge cache.
4. [Zone Domain Migration](./zone-domain-migration.md): upgrade from legacy managed domains / inline route domains to the Zone model (automatic goose import), with backup, acceptance, and rollback notes.
5. [Pages Static Hosting Usage](./pages-usage.md): static project ZIP upload limits, SPA Fallback, and built-in API reverse proxy config.
6. [Tunnel & Intranet Penetration](./tunnel-usage.md): deploy Relay and Client for secure reverse penetration without a public IP.
7. [WAF Security Protection](./waf-usage.md): configure WAF rule groups; master IP allow/block lists, auto/subscription IP groups, geo restrictions, and PoW CC protection.
8. [WAF Auto IP Group Expressions](./waf-ip-group-expr.md): write auto IP group Expr rules; understand keyword meanings and preset rules.
9. [Uptime Kuma Monitoring Sync](./uptime-kuma.md): configure Uptime Kuma auto differential sync and monitor scope control.
10. [SSO Login Configuration](./sso.md): configure OIDC for third-party single sign-on (SSO).
11. [Troubleshooting](./troubleshooting.md): troubleshoot login, database, node sync, OpenResty, and edge cache hit issues by symptom.
12. [Credits](./credits.md): the excellent open-source projects and community acknowledgments this system depends on.

## Find by Role

| What you want to do | Recommended Entry |
| --- | --- |
| Get the admin panel running in 5 minutes | [Quick Start](./quick-start.md) |
| Publish your first reverse proxy config | [Publish First Configuration](./first-site.md) |
| Configure domain certs, reverse proxy, and edge cache | [Create a Reverse Proxy Config](./proxy-config.md) (incl. cache notes) |
| Static assets not hitting cache | [Troubleshooting · Edge Cache](./troubleshooting.md#edge-cache-hit-rate-anomalies) |
| Host an SPA or static website | [Pages Static Hosting Usage](./pages-usage.md) |
| Configure intranet penetration mapping | [Tunnel & Intranet Penetration](./tunnel-usage.md) |
| Configure anti-CC and IP group blocking | [WAF Security Protection](./waf-usage.md) |
| Write auto IP group rules | [WAF Auto IP Group Expressions](./waf-ip-group-expr.md) |
| Auto-sync monitored site status | [Uptime Kuma Monitoring Sync](./uptime-kuma.md) |
| Connect or reinstall a node Agent | [Access Agent](../deployment/agent.md) |
| Start the Server from source | [Start the Server](../deployment/server.md) |
| Configure OIDC login | [SSO Login Configuration](./sso.md) |
| Upgrade Server or Agent | [Upgrade & Maintenance](../deployment/upgrade.md) |
| Understand the architecture and release model | [System Architecture](../design/architecture.md) and [Agent & Publish Model](../design/agent-design.md) |
| See open-source references and acknowledgments | [Credits](./credits.md) |

## Doc Sections

`guide/` targets users and deployers with executable steps from install to daily operations.

`reference/` consolidates stable facts: config fields, commands, API response conventions, and repository structure.

`design/` targets maintainers and contributors, describing product boundaries, system architecture, the Agent & publish model, and engineering constraints. Before adding capabilities or changing boundaries, update the corresponding design doc first.
