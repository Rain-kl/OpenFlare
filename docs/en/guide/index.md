# User guides

Step-by-step operational tasks across the OpenFlare console, CLI, and edge nodes.

## Getting started & traffic routing

| Guide | Scope |
| --- | --- |
| [Quick start](./quick-start.md) | Start Server and connect your first `openflare-edge` node |
| [Configure a reverse proxy](./proxy-config.md) | Set up origins, domains, certs, site auth, caching, and sync checks |
| [TLS certificates and auto-renewal](./certificates.md) | Import PEM certs or automate ACME DNS-01 issuance |
| [OpenFlare Edge performance policies](./performance-policies.md) | Configure worker threads, upstream pools, and cache storage |
| [SmartDNS authoritative resolution](./smartdns.md) | Configure NS delegation, `openflare-dns` nodes, and steering policies |

## Security, integrations & operations

| Guide | Scope |
| --- | --- |
| [WAF security protection](./waf-usage.md) | Build WAF DAG workflows, attack protection rule sets, and allowlists |
| [PoW protection](./pow-usage.md) | Enable route PoW challenges and customize challenge pages |
| [WAF auto IP group syntax](./waf-ip-group-expr.md) | Write Expr rules over access logs to maintain IP groups |
| [Pages static hosting](./pages-usage.md) | Deploy static bundles from uploads, Remote URLs, or GitHub Releases |
| [Tunnels and intranet penetration](./tunnel-usage.md) | Configure relay pools, relays, and `openflared` for L7 and L4 services |
| [Uptime Kuma monitoring sync](./uptime-kuma.md) | Sync enabled sites into Uptime Kuma monitors |
| [SSO login configuration](./sso.md) | Integrate OIDC identity providers and account binding |
| [Network tools and quality matrix](./network-tools.md) | Run multi-location probes, ASN/DNS diagnostics, and quality simulations |
| [CLI and MCP](./automation.md) | Authorize `openflare` CLI and MCP for routine operations |
| [Rotate the session secret](./session-secret-rotation.md) | Re-encrypt stored credentials before rotating `app.session_secret` |
| [Troubleshooting](./troubleshooting.md) | Diagnose Server, login, edge node, HTTPS, and caching issues |

See [Deployment](../deployment/index.md) for installation, [Reference](../reference/index.md) for configuration and APIs, and [Architecture](../design/index.md) for internals.
