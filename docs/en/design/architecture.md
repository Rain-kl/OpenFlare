# System Architecture

You will learn: OpenFlare's overall architecture, the responsibility split of each core component (Server, Agent, OpenResty, Relay, Client), and the macro flow of the main data and request streams.

OpenFlare is a self-hosted OpenResty control plane. Physically it consists of the Server (control plane), the Agent (config landing), node-local OpenResty (data plane), intranet penetration components (Relay and OpenFlared, data-plane extensions), and the admin frontend.

---

## Traffic Path Overview

Depending on the website upstream type, OpenFlare supports three data-plane traffic paths:

### 1. Standard Reverse Proxy Path
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenResty (WAF, TLS, Rate Limit, optional origin error page)
  |
  | reverse proxy (proxy_pass)
  v
Origin Server (direct public/LAN upstream)
```

When the origin or gateway returns an error status in the configured list, a global custom/default HTML can be returned while keeping the real HTTP status; see [Origin Error Page Design](./origin-error-page.md).

### 2. Intranet Penetration Path
For origin services on firewall-restricted intranet servers:
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenResty (Agent host, TLS/WAF)
  |
  | proxy_pass http://localhost:vhost_port (Host header preserved)
  v
OpenFlareRelay (frps)              <-- same host as the Agent, provides relaying
  |
  | frp tunnel protocol (Host header routing)
  v
OpenFlared (frpc)                  <-- firewall-restricted intranet server
  |
  | HTTP/HTTPS forward
  v
Internal Service (192.168.x.x)
```

### 3. Pages Static Hosting Path
For pre-built SPAs or static site hosting:
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenResty (Agent, TLS/WAF)
  |
  +---> [static serving] root/try_files ---> Agent local Pages deployment dir
  |
  +---> [API proxy] proxy_pass ---> backend API service (if API proxying enabled)
```

---

## Component Responsibilities

| Component | Responsibility | Detailed Design Reference |
| --- | --- | --- |
| **Server** | admin UI/API, control-plane state persistence, config compilation/rendering, release versioning, Pages deployment package storage, Cloudflare A-record pointing, access-log storage and business traffic aggregation, Uptime Kuma monitoring sync, login CAPTCHA protection | [Agent & Publish Model](./agent-design.md) / [Cloudflare DNS Pointing Design](./cloudflare-pointing.md) / [Edge Observability & Business Traffic Stats](./observability-design.md) / [Uptime Kuma Sync Design](./kuma-design.md) / [Login CAPTCHA Design](./login-captcha.md) |
| **Agent** | periodic heartbeat & WS sync, static package pull/extraction, OpenResty config write/validate/reload and self-healing; observability reports only access details and host/health readings, no business pre-aggregation | [Agent & Publish Model](./agent-design.md) / [Edge Observability & Business Traffic Stats](./observability-design.md) |
| **OpenResty** | receives real traffic; executes WAF filtering, PoW protection, Basic Auth, static/reverse-proxy serving, and optional origin error pages | [WAF Design](./waf-design.md) / [Pages Design](./pages-design.md) / [Origin Error Page Design](./origin-error-page.md) |
| **Relay** | deployed on edge nodes; manages the `frps` daemon lifecycle and accepts heartbeat-dispatched penetration relay configs | [Tunnel Design](./tunnel-design.md) |
| **OpenFlared** | deployed in the intranet; manages the `frpc` process group, establishes reverse tunnels to multiple Relays, reports connection state | [Tunnel Design](./tunnel-design.md) |

---

## Component Architecture and Division

### 1. Server (control plane)
The Go backend at the repo root (module `github.com/Rain-kl/Wavelet`) is the OpenFlare control plane, built on the Wavelet full-stack scaffold:
* Provides admin REST APIs (`/api/v1/d/*`) authenticated via **Session Cookie**, with optional `X-Access-Token`.
* Edge node protocols go through `/api/v1/agent|relay|tunnel/*`, authenticated with `X-Agent-Token` / `X-Tunnel-Token` respectively.
* Contains the config Compiler, uniformly compiling DB rules, certs, and global params into immutable config snapshots and OpenResty physical config file text.
* Uniformly receives Pages local uploads, Remote URLs, and public GitHub Release pre-built artifacts, completing source checks, restricted downloads, archive validation, and immutable deployments; manual uploads create candidates awaiting explicit activation, persistent-source sync creates-or-loads and atomically activates. The Server offers controlled latest-download endpoints to Agents; the internal scanner handles limited GitHub latest checks, lease recovery, optional auto-publish, and orphan upload compensation; the generic task management entry can't modify this schedule. Future repo source builds are extended by a standalone Server build executor; the Agent never executes third-party fetch or build commands.
* Provides the optional Cloudflare DNS pointing control plane: maintains group desired state with ZoneDomains as members, idempotently syncing a single A record to the current active node IPv4 via Asynq; node IP changes only best-effort enqueue; no auto-failover in phase 1.
* Backend integration with the Uptime Kuma monitoring sync service auto-maintains HTTP probe tasks for available sites.
* Startup entry: root `main.go` + `internal/cmd/` (`api` / `worker` / `scheduler` / `all`); OpenFlare business in `internal/apps/openflare/`, edge protocol handling in `internal/apps/openflare/{agent,relay,flared}/`.
* *See: [Agent & Publish Model](./agent-design.md) and [Uptime Kuma Sync Design](./kuma-design.md)*

### 2. Agent (config landing)
`openflare-agent` is the daemon running on the node:
* Maintains periodic heartbeats with the control plane after startup, receiving real-time config release broadcasts via the optional WebSocket.
* Pulls the latest active version's config files and certs, writes them locally, and performs safe validation via `openresty -t` before a smooth reload.
* Handles Pages deployment package download, SHA-256 validation, and extraction switching locally.
* *See: [Agent & Publish Model](./agent-design.md)*

### 3. OpenResty (data plane)
Receives visitor traffic and performs final business landing:
* Traffic entry, supporting HTTP/2, HTTP/3 (QUIC), and dynamic TLS certificate binding.
* Embeds Lua logic filtering WAF rules and verifying PoW challenges efficiently in the `access_by_lua` phase, followed by connection/rate limits and basic caching (policy in [Edge Cache Strategy Design](./edge-cache-design.md)).
* *See: [WAF Design](./waf-design.md) and [Pages Static Hosting Design](./pages-design.md)*

### 4. Relay and OpenFlared (tunnel components)
Extend data-plane reverse penetration:
* `openflare-relay` guards the local `frps`, accepts Server config dispatch, and auto-updates the relay port.
* `openflared` guards a group of `frpc` client processes in the intranet for nearest multi-relay connections and HA disaster recovery.
* *See: [Tunnel Design](./tunnel-design.md)*

---

## Data and Request Flow Overview

### 1. Config Release and Sync Flow
```text
admin modifies config -> release new version -> generate globally unique Checksum active version
                                 |
              +------------------+------------------+
              | (WebSocket broadcast or periodic Heartbeat)      |
              v                                     v
       [edge node Agent]                        [intranet OpenFlared]
  pull latest OpenResty config/certs            pull latest Tunnel mapping config
  incrementally pull/extract Pages packages     generate/rewrite frpc.toml
  validate config and smooth reload             smooth reload or spawn frpc
  report apply state (Success / Error)          report tunnel connection state and metrics
```
* *Fine-grained sync/self-healing timing and the rollback model: [Agent & Publish Model](./agent-design.md)*

### 2. Static Hosting and API Proxy Flow
* Static assets are extracted to `projects/{project_id}/current` on the Agent node (pulled per project latest, only the newest package kept); OpenResty serves static resources at the edge via `root`/`index`/`try_files`.
* With API proxying enabled, OpenResty rewrites and forwards (`proxy_pass`) API requests to the backend dynamic API based on the site's `api_proxy_path` (e.g. `/api`).
* Admin operations and the internal scanner only generate constrained artifact candidates, reusing the unified inspect, `upload.Ingest`, and deployment pipeline. Manual uploads create a new inactive candidate; persistent-source sync/scanner creates-or-loads and atomically activates. A future repository build executor can only emit into the same artifact pipeline; the Agent is always just an active-deployment consumer.
* *Package validation, extraction escape defense, and Nginx rule rendering: [Pages Static Hosting Design](./pages-design.md)*

### 3. WAF Security Filtering Flow
* The WAF engine is embedded in the OpenResty request lifecycle.
* WAF rules are orchestrated as a visual DAG on the control plane and compiled into a runtime graph at release; after an OpenResty reload each Worker loads it once, and subsequent requests only traverse the in-memory object.
* Global rules always run first; route-bound rules execute in explicit order; reaching "pass" in the current rule continues to the next, reaching "block" immediately returns that node's configured block response.
* IP group members hot-update independently: a coordinating worker checks the checksum every 5 seconds, loading the full snapshot only on change; each Worker's request path always reads the local in-memory object.
* *IP group sources and sync: [WAF Design](./waf-design.md); graph model, execution semantics, release constraints: [WAF Orchestration Rule Design](./waf-orchestration-design.md)*

### 4. Edge Observability and Business Traffic Stats Flow
```text
OpenResty access.log (business facts)
        |
        | Agent tails incremental details (no sum/count/uniq)
        v
Server stores via logstore (current log primary DB: PostgreSQL / SQLite / ClickHouse)
        |
        +---> global aggregation --> dashboard "data provided / requests / UV"
        +---> host∈Zone --> Zone "data provided" etc. (same semantics)
        +---> node_id filter --> node business volume

host /proc NIC, CPU etc. --> Agent reading snapshots --> host resource trends (displayed separately from business delivery)
OpenResty health and connections --> edge health (instant, not 24h business totals)
```
* **Principle**: the Agent reports only facts; the Server interprets facts; access logs are the single truth for business traffic. `openresty_tx` and "data provided" must not run on dual tracks.
* *Transport model, examples, and collection frequency: [Observability Transport Model](./observability-transport-model.md); field convergence and migration: [Edge Observability & Business Traffic Stats](./observability-design.md)*

### 5. Cloudflare DNS Pointing Flow

```text
admin configures connection/group/member -> Server persists desired state -> Asynq sync tasks
                                                        |
                                                        v
                                              Cloudflare Zone / DNS API
                                                        |
                                                        v
                                    single A record -> active_node IPv4

node IP manually updated or Agent heartbeat change --------------------> best-effort enqueue per node
```

* The Cloudflare module only manages cached or taken-over uniquely-named A records; it doesn't extend the Zone core into an authoritative DNS control plane. On multiple same-name A records it stops syncing and asks the admin to clean up in Cloudflare.
* Group backup/active nodes are reserved for later failover; phase 1 fixes the primary node and doesn't auto-switch on heartbeat offline.
* *Connection, model, idempotent sync, and phasing: [Cloudflare DNS Pointing Design](./cloudflare-pointing.md)*

---

## Core Objects

Current core system entities include:

* **Reverse proxy & config**: `zones` (root-domain management boundary), `zone_domains` (explicit domains with cert/route association), `proxy_routes` (route policy), `origins`, `config_versions`, `tls_certificates`. See [Zone & Domain Resource Design](./zone-design.md).
* **Cloudflare DNS pointing**: `of_cf_connections` (global connection), `of_cf_pointing_groups` (primary/backup/active nodes and default orange-cloud), `of_cf_pointing_members` (ZoneDomain members, record cache, sync state). See [Cloudflare DNS Pointing Design](./cloudflare-pointing.md).
* **Pages static hosting**: `of_pages_projects`, `of_pages_project_sources` / `of_pages_project_source_runtime` (mutable source config and runtime), `of_pages_deployments` (immutable deployments), `of_pages_deployment_files` (deployment file manifests).
* **Nodes & tunnels**: `nodes`, `tunnels` (tunnel clients), `node_system_profiles`, `apply_logs`.
* **WAF & security**: `waf_rule_groups`, `waf_ip_groups`, `waf_rule_group_bindings` (site WAF bindings).
* **System & accounts**: `acme_accounts`, `dns_accounts`, `geoip_update_configs`.

---

## Key Design Decisions

| Decision | Reason |
| --- | --- |
| Full config versions instead of online patching | stable boundaries for preview, activation, history, and rollback; consistent node state |
| Agent active pull | the Server needs no SSH access, lowering security risk; supports HTTP/WebSocket dual-protocol switching |
| Globally single active version | lowers control-plane complexity, keeps all nodes consistent by default; stable one-click second-level rollback |
| Zone domains separated from route policy | Zones provide the root-domain entry and domain boundaries; routes still reuse the same site-level policy and bind certs per domain |
| Cloudflare pointing independent of the Zone core | ZoneDomains only provide explicit FQDNs; the Cloudflare module drives single A records from DB desired state without widening Zones into a general DNS control plane |
| Intranet penetration integrated on frp | reuses a mature tunnel protocol, avoiding self-built tunnel stability risks; its Vhost mechanism natively fits reverse-proxy routes |
| Runtime config decoupled from the control store | WAF rules compile at release and load with the OpenResty reload; dynamic IP groups refresh independently via checksum-driven memory snapshots |
| Access logs as the single truth for business traffic | the Agent forbids business pre-aggregation; dashboard and Zone share Server-side aggregation, avoiding openresty_tx vs bytes_sent dual tracks |
| Business delivery / edge health / host capacity layered | data provided ≠ host NIC outbound ≠ OpenResty connections; UI and API name and section them separately |
| Pages artifacts separated from repo builds | current sources only import pre-built artifacts; future checkout/build happens in a Server-isolated executor reusing the artifact pipeline; the Agent never runs third-party builds |

---
