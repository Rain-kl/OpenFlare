# Product Boundaries

You will learn: What OpenFlare is, what problems it solves, who the target audience is, what current stable features are available, and which design boundaries cannot be bypassed during implementation.

OpenFlare is a self-hosted OpenResty control plane designed for single-team or single-organization internal operations. It solves the problems of decentralized management of reverse proxy configurations, node synchronization, certificate hosting, configuration publication and rollback, and basic observability.

## Project Positioning

OpenFlare is suitable for teams that need to centrally manage multiple OpenResty proxy nodes:

* Wanting to maintain reverse proxy website configurations using a management dashboard.
* Wanting every configuration change to have a complete version history, preview, activation, and rollback support.
* Wanting nodes to actively synchronize configurations, rather than the control plane SSHing into nodes to execute commands.
* Wanting to manage TLS certificates, domain assets, node statuses, and basic access analytics in a single system.

OpenFlare is currently not positioned as a general-purpose logging platform, service mesh, Kubernetes Ingress Controller, or multi-tenant cloud platform.

## Current Capabilities

| Capability | Description |
| --- | --- |
| Reverse Proxy Rules | Uses website configuration as the aggregation boundary, supporting multiple domains and origin settings. |
| Website-level Config | One rule corresponds to one website, which can bind one or more domains and share site-level configurations. |
| Origin Management | Maintains a lightweight origin directory and allows websites to save renderable origin snapshots. |
| Config Versioning | Supports previews, publishing, activation, immutable history, and rollbacks. |
| Agent Sync | Supports registration, heartbeats, synchronization, application result reporting, and self-updating. |
| OpenResty Hosting | Manages main config templates, performance parameters, cache parameters, and Lua resources. |
| HTTPS/TLS | Hosts certificate and domain assets, binding certificates on a per-domain basis. |
| WAF | Maintains IP/CIDR block blacklists/whitelists, IP groups, and country-level geographic access controls at both global and site-specific levels. |
| Basic Observability | Aggregates node requests, resource snapshots, health events, and access analytics. |
| Node Management | Manages node status, token systems, and deployment/update lifecycles. |
| Admin UI | Next.js-based official management dashboard. |
| Auth Source Login | Supports configuring GitHub OAuth and standard OIDC login portals, allowing third-party accounts to bind to existing local users. |
| Intranet Penetration | Securely exposes intranet HTTP services to the public internet using TunnelRelay nodes and the OpenFlared client, reusing the Agent's HTTPS/WAF capabilities. |

Default Working Model:

* All nodes consume the same globally activated configuration version.
* The Server stores configurations and state, and does not directly SSH to manage nodes.
* The Agent is the only controlled entry point on the node side.
* TunnelRelay nodes run both the Agent (OpenResty) and the Relay (frps manager) to provide intranet penetration relays.
* The OpenFlared client runs inside the intranet, managing the frpc process to connect to the Relay and forward traffic to intranet services.

## Typical Use Cases

| Scenario | Description |
| --- | --- |
| Unified Entrance | Exposes multiple internal HTTP services via a unified domain and TLS certificate. |
| Multi-Node Sync | Multiple OpenResty nodes consume the same active configuration version. |
| Change Review | View previews or diffs before publishing, keeping an immutable history post-publish. |
| Rapid Rollback | Re-activate an older version, letting the Agent pull and apply it. |
| Certificate Hosting | Bind TLS certificates to different domains under the same website. |
| Observability | Check node health status, aggregated requests, traffic analytics, and health events. |
| Intranet Penetration | Exposes intranet HTTP services that are not directly reachable from the public internet using Tunnels, benefiting from HTTPS, WAF, and all other protections. |

## Website Configuration Constraints

`proxy_routes` is the aggregate object for "website configurations". One record corresponds to one website, which can bind one or more domains and share a set of site-level configurations.

Constraints:

* `proxy_routes.site_name` is the unique business identifier of the website.
* `proxy_routes.domains` must contain at least one domain, and `domains[0]` is treated as the primary domain.
* Any domain can globally belong to only one `proxy_routes`.
* Site-level rate limits, reverse proxies, and caching configurations are shared by the site, with no per-domain differences allowed within the same website.
* HTTPS allows binding certificates on a per-domain basis within the same site.

## Origin & Upstream Constraints

`origins` serve the reuse of the origin directory, storing only the origin address, display name, and remarks, without carrying protocols, ports, paths, weights, or health check policies. `proxy_routes` can optionally associate with an `origins` record, but the rule internally still saves a complete upstream snapshot for rendering.

Upstream Constraints:

* `proxy_routes` must contain at least one upstream address (for direct type `direct`), or be associated with a Tunnel (for intranet penetration type `tunnel`).
* Multi-upstream load balancing is uniformly rendered into a named `upstream` with keepalive enabled.
* A single upstream is allowed to carry a base path or query, which is appended in `proxy_pass`. Multi-upstream is strictly limited to pure `scheme://host[:port]` structures, and all upstreams in the same rule must use the same protocol.
* `proxy_routes.origin_host` is an optional field used to override the `Host` header during back-to-source requests.
* All direct upstream addresses must be valid `http://` or `https://` URLs.
* Intranet penetration upstreams must associate with a valid `tunnel_id` and specify the intranet target address and protocol.

## Intranet Penetration Constraints

OpenFlare implements intranet penetration through TunnelRelay nodes and the OpenFlared client, built on top of frp (Fast Reverse Proxy).

### Node & Component Model

**Node Types**:

* `nodes.node_type` distinguishes the node type: `edge_node` (edge node, default) and `tunnel_relay` (tunnel relay).
* TunnelRelay nodes run both the Agent (OpenResty) and the Relay (frps manager) concurrently, sharing the same `agent_token`.
  - The Agent is responsible for HTTPS termination, WAF protection, caching, and rate limiting.
  - The Relay manages the frps process, providing tunnel relay services for intranet clients.
* TunnelRelay nodes introduce new fields: `node_type`, `relay_bind_port` (frpc connection port, default 7000), `relay_vhost_http_port` (HTTP Vhost port, default 8080), `relay_auth_token` (automatically generated), `relay_status`, etc.

**Tunnel Client**:

* The `tunnels` table independently stores intranet penetration client registration info and is decoupled from the `nodes` system.
* Each Tunnel has a unique `tunnel_id` (format `tun-<32hex>`) and `tunnel_token` (client authentication credential).
* The OpenFlared client runs inside the intranet, is not exposed to the public internet, uses `tunnel_token` for authentication, and communicates with the Server via `/api/flared/*` endpoints.
* An OpenFlared client can connect to multiple Relays simultaneously for high availability.

### Upstream Type Expansion

The upstream configuration of `proxy_routes` is divided into two types, distinguished by the `upstream_type` field:

* **Direct Upstream (`direct`, default)**: Forwards traffic directly to the origin address, behaving exactly like the existing mechanism.
* **Intranet Penetration Upstream (`tunnel`)**: Forwards traffic to the intranet service via a TunnelRelay node.
  - Must specify `tunnel_id` (associated with the `tunnels` table).
  - Must specify `tunnel_target_addr` (intranet target address, e.g., `192.168.1.100:8080`) and `tunnel_target_protocol` (`http` or `https`).
  - During publication, the Server automatically replaces the upstream address with `http://127.0.0.1:{relay_vhost_http_port}`.

### Traffic Paths & Protocols

**Complete Data Plane Traffic Path**:

```
Browser → OpenResty (Agent, TLS/WAF)         [TunnelRelay Node]
       ↓
     frps (Relay, HTTP Vhost Routing)        [TunnelRelay Node, 127.0.0.1:{vhost_port}]
       ↓
   frp Tunnel Protocol (Host Header Routing)
       ↓
     frpc (Client, Multi-process)            [Intranet Server]
       ↓
    Intranet Service (192.168.x.x:port)
```

**Key Features**:

* frps uses the HTTP Vhost single-port reuse mechanism; all HTTP tunnels share one `vhost_port`, automatically routed to the corresponding frpc based on the Host header.
* The Agent preserves the original `Host` header, which frps uses to match the virtual host.
* Each tunnel corresponds to a single `proxy_routes` and can bind multiple domains.
* The OpenFlared client manages an independent frpc process for each connected Relay, transmitting multiple HTTP proxy definitions via a single frp tunnel.

### Configuration Sync Model

The publication process generates two types of configuration version data simultaneously, linked by a single `config_version` version number:

* **Agent-side Config**: OpenResty main configuration + route configurations + WAF rules. If a tunnel upstream is included, it is automatically rendered as a `http://127.0.0.1:{vhost_port}` upstream.
* **Tunnel-side Config**: Relay list + frpc proxy definitions. Versioned alongside the publishing process; changes are hot-reloaded using `frpc reload` first.
* **Relay Config**: Dispatched via heartbeat responses, relatively static, and not included in the versioned publishing flow.

### Tunnel Design Constraints

* Only HTTP protocol tunnel traffic is supported (keeping TCP/UDP tunnels extensible); separate TCP/UDP port allocation is not supported for now.
* The DNS for domains using Tunnel upstreams should resolve to the designated TunnelRelay node.
* frp binaries (v0.61+) are packaged and provided by the system deployment script or container images.

## HTTPS Constraints

`proxy_routes.domain_cert_ids` is used to record the domain-certificate bindings parallel to `domains`; a value of `0` means the domain does not have HTTPS enabled and stays HTTP-only.

During rendering:

* Domains with certificates are grouped by certificate and output as independent `443 ssl` `server` blocks.
* Domains without certificates bound must not be automatically routed to HTTPS.
* All domains in `proxy_routes.domains` must be kept in the same site configuration to avoid being split across version snapshots.

## WAF Constraints

WAF centers around rule groups. The system provides a single global rule group (applied to all sites by default), on top of which websites can overlay multiple custom rule groups.

Core Capabilities:

* Supports individual IP / CIDR block whitelists and blacklists.
* Supports IP group references (including manual, automatic Expr calculated, and URL subscribed IP groups).
* Supports GeoIP-based country/region level admission filtering.
* Supports custom interception responses for rule groups (custom status codes and interception HTML pages, default is `418`).

IP Group & Judgment Constraints:

* **Runtime Decoupling**: The WAF runtime only reads local JSON files and does not access the Server database; configuration versions only store referenced IP group IDs. IP group members are synchronized via MD5 checksum differences and WebSocket push notifications, achieving hot activation without reloading Nginx.
* **Built-in Expr Rules**:
  * High-frequency 404 scanning block: `request_count > 100 && status_404_ratio >= 0.8`
  * Malicious IP direct probe: `ip_host_count > 50 && ip_host_ratio > 0.5`
* **Decision Priority**: The whitelist has absolute priority. If it does not match the whitelist, the blacklist funnel is triggered (global rule group first, custom groups matched in ascending ID order).
* GeoIP resolution depends on the local MaxMind database; if GeoIP is anomalous, region rules are automatically ignored and must not disrupt the availability of IP rules and the main reverse proxy chain.

## Authentication Source Constraints

`auth_sources` uniformly supports `github` and `oidc` login configurations. `external_accounts` stores bindings between third-party accounts and local users. Logic for first-time third-party login:

* If already bound, directly authorize login; if there is an active local session, automatically bind.
* If unbound and registration is enabled, automatically create a local account; if registration is closed, require the user to provide an existing local username and password to establish the association.

## Version & Observability Constraints

* `config_versions` must save the complete snapshot, rendering result, and `checksum`.
* Globally, only one version can be active at a time.
* Rollback is achieved by re-activating an older version.
* `nodes` only carry control plane state and low-frequency summaries; they do not carry high-frequency observability facts.
* Metrics, trends, and access analytics prioritize server-side aggregation rather than client-side temporary statistics.
* Access detail logs are only retained within a controlled time window, not evolving into a general logging platform.

## Documentation Maintenance Principles

* Update this document when the product range or system boundaries change.
* Update [System Architecture](./architecture.md) when the system structure or module responsibilities change.
* Update [Agent & Publish Model](./agent-design.md) when the publishing, synchronization, rollback, or Agent model changes.
* Update [Development Constraints](../../guideline/Constraints.md) when developer constraints, code specifications, or API conventions change.
* Update README and [Deployment Instructions](../../deployment/deployment.md) when deployment methods change.
* Update [Configurations Reference](../reference/configuration.md) when configuration items change.
* Completed phases should no longer be backfilled as "version plans".
* Before starting a new phase, complement the design first, then proceed to implementation.
