# System Architecture

OpenFlare consists of the Server control plane, the OpenFlare Edge data plane, tunnel components (Relay and OpenFlared), and the management console frontend. For edge synchronization, validation, and runtime application workflows, see [OpenFlare Edge node Design](./pingora-edge-design.md).

---

## Traffic Path Overview

Depending on the website upstream type, OpenFlare supports three distinct data plane traffic paths:

### 1. Standard Reverse Proxy Traffic Path
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenFlare Edge (WAF, TLS, Rate Limit, optional Origin Error Page)
  |
  | reverse proxy
  v
Origin Server (direct public or private upstream)
```

When an origin server or gateway returns error status codes matching the configured list, OpenFlare returns global custom or default HTML while preserving the real HTTP status code. See [Origin Error Page Design](./origin-error-page.md).

### 2. Intranet Penetration Traffic Path
Suitable for routing traffic to origin services hosted on private or firewall-restricted networks:
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenFlare Edge (Agent host, TLS/WAF)
  |
  | reverse proxy to http://localhost:vhost_port (Host header preserved)
  v
OpenFlareRelay (frps)              <-- co-located with Agent, provides relay
  |
  | frp tunnel protocol (Host header routing)
  v
OpenFlared (frpc)                  <-- intranet server
  |
  | HTTP/HTTPS forward
  v
Internal Service (192.168.x.x)
```

### 3. Pages Static Hosting Traffic Path
Suitable for pre-built Single Page Applications (SPAs) or static website hosting:
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenFlare Edge (Agent, TLS/WAF)
  |
  +---> [Static Serving] reads deployed files ---> Agent local Pages deployment directory
  |
  +---> [API Proxy] reverse proxy ---> Backend API service (if API proxying enabled)
```

---

## Component Responsibilities

| Component | Responsibility | Detailed Design Reference |
| --- | --- | --- |
| **Server** | Admin UI/API, control-plane state persistence, configuration compilation and auto-sync, Pages deployment asset storage, Cloudflare A-record pointing, access log ingestion and traffic aggregation, Uptime Kuma monitoring sync, and login captcha protection | [OpenFlare Edge node Design](./pingora-edge-design.md) / [Cloudflare DNS Pointing Design](./cloudflare-pointing.md) / [Edge Observability & Business Traffic Stats](./observability-design.md) / [Uptime Kuma Sync Design](./kuma-design.md) / [Login Captcha Design](./login-captcha.md) |
| **OpenFlare Edge** | Ingests live traffic, evaluates WAF rules, PoW defense, Basic Auth, static file and reverse proxy serving, and optional origin error pages | [WAF Design](./waf-design.md) / [Pages Design](./pages-design.md) / [Origin Error Page Design](./origin-error-page.md) |
| **Relay** | Deployed on edge nodes, manages `frps` daemon lifecycle, and receives dynamically dispatched tunnel relay configuration via heartbeats | [Tunnel Design](./tunnel-design.md) |
| **OpenFlared** | Deployed in private networks, manages `frpc` process groups, maintains reverse tunnels across multiple Relays, and reports connectivity | [Tunnel Design](./tunnel-design.md) |

---

## Component Architecture and Division of Labor

### 1. Server (Control Plane)
The Go backend at the repository root (module `github.com/Rain-kl/Wavelet`) serves as the OpenFlare control plane, built on the Wavelet full-stack scaffold:
* Provides admin REST APIs (`/api/v1/d/*`) authenticated via **Session Cookie**, with optional `X-Access-Token`.
* Edge node protocols go through `/api/v1/agent|relay|tunnel/*`, authenticated with `X-Agent-Token` / `X-Tunnel-Token` respectively.
* Contains the configuration Compiler, uniformly compiling DB rules, certificates, and global parameters into immutable configuration snapshots and required runtime resources.
* Uniformly receives Pages local uploads, Remote URLs, and public GitHub Release pre-built artifacts, completing source checks, restricted downloads, archive validation, and immutable deployments; manual uploads create candidates awaiting explicit activation, while persistent-source sync creates-or-loads and atomically activates. The Server provides controlled latest-download endpoints to edge nodes; an internal scanner handles limited GitHub latest checks, lease recovery, optional auto-publish, and orphan upload compensation; generic task management endpoints cannot modify this schedule. Future repository source builds are extended via an isolated Server build executor; edge nodes never execute third-party fetch or build commands.
* Provides the optional Cloudflare DNS pointing control plane: maintains group desired states with ZoneDomains as members, idempotently syncing a single A record to the active node IPv4 via Asynq; node IP changes are queued best-effort; phase 1 does not execute automatic failover.
* Integrates backend Uptime Kuma monitoring synchronization, automatically maintaining HTTP probe tasks for active sites.
* Startup entry: `cmd/server/main.go` + `cmd/server/cli/` (`api` / `worker` / `scheduler` / `all`); OpenFlare business logic resides in `internal/apps/openflare/`, Server-side edge protocol handling in `internal/apps/openflare/{agent,relay,flared}/`, and edge runtimes in `edge/` and `internal/edge/{common,relay,flared}/`.
* *See: [OpenFlare Edge node Design](./pingora-edge-design.md) and [Uptime Kuma Sync Design](./kuma-design.md)*

### 2. OpenFlare Edge (Data Plane)
`openflare-edge` manages listeners, snapshots, and Pages assets, executing HTTP proxying, HTTP/2, TLS, WAF, PoW, site authentication, and caching within a single unified runtime. Nodes receive update notifications via WebSockets and synchronize site snapshots, IP groups, and performance policies through heartbeat manifests.

### 3. Relay and OpenFlared (Tunnel Components)
Extend data-plane reverse penetration capabilities:
* `openflare-relay` manages the local `frps` daemon, receives Server configuration dispatches, and automatically updates relay ports.
* `openflared` manages a group of `frpc` client processes within internal networks, establishing nearest multi-relay connections and high-availability disaster recovery.
* *See: [Tunnel Design](./tunnel-design.md)*

---

## Data and Request Flow Overview

### 1. Configuration Save and Sync Flow
```text
Admin saves config -> auto-compiles and updates site snapshot by digest
                                 |
              +------------------+------------------+
              | (WebSocket broadcast or periodic Heartbeat)
              v                                     v
       [Edge Node]                             [Intranet OpenFlared]
  pull site snapshot and certs by hash    pull latest Tunnel mapping config
  pull/extract Pages packages             generate/rewrite frpc.toml
  verify and atomically swap in memory    smooth reload or spawn frpc
  report apply state (Success / Error)    report tunnel connection state and metrics
```
* *For fine-grained synchronization timing and the rollback model, see [OpenFlare Edge node Design](./pingora-edge-design.md).*

### 2. Static Hosting and API Proxy Flow
* Static assets are extracted to `projects/{project_id}/current` on the edge node (pulled per project latest, keeping only the latest package); OpenFlare Edge serves static files directly from local storage.
* When API proxying is enabled, OpenFlare Edge rewrites and forwards (`reverse proxy`) API requests to backend dynamic endpoints according to the site's configured `api_proxy_path` (e.g. `/api`).
* Administrator operations and internal scanners generate restricted artifact candidates, reusing the unified inspection, `upload.Ingest`, and deployment pipeline. Manual uploads create unactivated candidates; persistent source syncs create-or-load and atomically activate. Future repository build executors will emit artifacts into the same pipeline; edge nodes remain consumers of active deployments.
* *See [Pages Static Hosting Design](./pages-design.md).*

### 3. WAF Security Filtering Flow
* The WAF engine evaluates requests during the OpenFlare Edge request phase.
* WAF rules are orchestrated as visual DAGs on the control plane and compiled into runtime graphs upon publishing; edge nodes load and execute them via in-memory objects.
* Global rules precede route-bound rules, which execute in explicit order. When an evaluation reaches "pass", processing proceeds to the next rule; reaching "block" immediately returns the node-configured blocking response.
* IP group membership updates dynamically: the control plane notifies nodes to verify checksums, and nodes update only modified IP groups.
* *See [WAF Design](./waf-design.md) and [WAF Orchestration Rule Design](./waf-orchestration-design.md).*

### 4. Edge Observability and Business Traffic Statistics Flow
```text
OpenFlare Edge access logs (business ground truth)
        |
        | nodes report log entries via heartbeats
        v
Server ingests via logstore (current log primary: PostgreSQL / SQLite / ClickHouse)
        |
        +---> Global aggregation --> Dashboard "Data Provided / Requests / Unique Visitors"
        +---> host in Zone --------> Zone "Data Provided" etc. (identical semantics)
        +---> node_id filter ------> Node business traffic volume

Host /proc network and CPU metrics --> Agent snapshot readings --> Host resource trends (isolated from business traffic)
OpenFlare Edge health and connection counts --> Edge health (instantaneous readings, not 24h business totals)
```
* **Principle**: The edge node reports facts only; the Server interprets facts; access logs are the single truth for business traffic. `openresty_tx` and "Data Provided" must not run on dual tracks.
* *See: [Observability Transport Model](./observability-transport-model.md) and [Edge Observability & Business Traffic Stats](./observability-design.md).*

### 5. Cloudflare DNS Pointing Flow
```text
Admin configures connection/groups/members -> Server persists desired state -> Asynq sync task
                                                                                   |
                                                                                   v
                                                                         Cloudflare Zone / DNS API
                                                                                   |
                                                                                   v
                                                               single A record -> active_node IPv4

Node IP manual update or Agent heartbeat change ------------------------> Node-level best-effort enqueue
```

* The Cloudflare module manages only the single A record of the same name under its control, without expanding the Zone core into an authoritative DNS control plane. When multiple A records of the same name exist, synchronization halts until cleaned up in Cloudflare.
* Group backup nodes and active nodes are reserved for future failover; phase 1 fixes usage to the primary node without automatic failover based on heartbeat offline status.
* *See [Cloudflare DNS Pointing Design](./cloudflare-pointing.md).*

---

## Core Objects

Core system entities include:

* **Proxy and Configuration**: `zones` (root domain boundary), `zone_domains` (explicit domain and cert/route bindings), `proxy_routes` (route policies), `origins` (origin servers), `config_versions` (configuration versions), `tls_certificates` (certificates). See [Zone & Domain Resource Design](./zone-design.md).
* **Cloudflare DNS Pointing**: `of_cf_connections` (global connections), `of_cf_pointing_groups` (primary/backup/active nodes and default orange-cloud), `of_cf_pointing_members` (ZoneDomain members, record cache, and sync status). See [Cloudflare DNS Pointing Design](./cloudflare-pointing.md).
* **Pages Static Hosting**: `of_pages_projects` (Pages projects), `of_pages_project_sources` / `of_pages_project_source_runtime` (source configuration and runtime state), `of_pages_deployments` (immutable deployments), `of_pages_deployment_files` (deployment file manifests).
* **Nodes and Tunnels**: `nodes` (nodes), `tunnels` (tunnel clients), `node_system_profiles` (system profiles), `apply_logs` (application logs).
* **WAF and Security**: `waf_rule_groups` (WAF rule groups), `waf_ip_groups` (WAF IP groups), `waf_rule_group_bindings` (site WAF bindings).
* **System and Accounts**: `acme_accounts` (ACME accounts), `dns_accounts` (DNS accounts), `geoip_update_configs` (GeoIP update configurations).

---

## Key Design Decisions

| Decision | Rationale |
| --- | --- |
| Complete site snapshots | Provides consistent boundaries for validation, atomic swapping, and local recovery |
| Active pull by edge nodes | Eliminates Server SSH requirements, lowering security risks; enables flexible HTTP and WebSocket switching |
| Single active version | Simplifies control plane state and ensures consistency across nodes; provides instant one-click rollback |
| Separation of Zone domains and route policies | Zone defines root domain boundaries; routes reuse site policies and bind certificates per domain |
| Cloudflare pointing independent of Zone core | ZoneDomain provides FQDNs; Cloudflare module synchronizes single A records from desired state without expanding Zone into a general DNS plane |
| Intranet penetration based on frp | Reuses proven tunnel protocols to avoid custom protocol risks; vhost routing naturally matches reverse proxy routes |
| Decoupled runtime configuration and control DB | WAF rules compile upon publishing and load atomically with site snapshots; dynamic IP groups refresh independently via checksum-driven in-memory snapshots |
| Access logs as the single truth for business traffic | Edge nodes forbid business pre-aggregation; dashboard and Zone share Server-side aggregation, avoiding dual tracks between openresty_tx and bytes_sent |
| Layered separation of business traffic / edge health / host resources | Data Provided != host network egress != OpenFlare Edge connection counts; UI and APIs maintain distinct naming and presentation |
| Pages artifact and repository build separation | Existing sources import pre-built packages; future checkout/build pipelines run in isolated Server executors and reuse the artifact pipeline; edge nodes do not execute third-party builds |
