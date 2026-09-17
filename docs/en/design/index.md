# Product Boundaries

You will learn: what OpenFlare is, its current stable capabilities, and the core product boundaries and repository structure layout you must follow when developing.

OpenFlare is a self-hosted OpenResty control plane for single-team or single-organization internal operations.

---

## Project Positioning

OpenFlare suits teams that need to centrally manage multiple OpenResty proxy nodes, with this positioning:
* **Control/landing separation**: the Server control plane doesn't SSH into proxy nodes; Agents actively pull versions and apply them.
* **Immutable config release**: full config versions are used for preview, release, activation, and one-click rollback.
* **Integrated gateway hosting**: website reverse proxying, automatic TLS certificate issuance/renewal, WAF protection, intranet penetration (Tunnel), and Pages static hosting are integrated into one control plane.

**Not this product's positioning**: multi-tenant cloud platforms, Kubernetes Ingress Controllers, service meshes, or general log platforms.

---

## Current Capabilities

| Capability | Description | Detailed Design/Usage |
| --- | --- | --- |
| **Reverse proxy config management** | website rules (Proxy Route) as the aggregation boundary; multi-domain and multi-upstream load balancing | [Create a Reverse Proxy Config](../guide/proxy-config.md) |
| **Origin error page** | globally configurable: matching origin/gateway status codes return OpenFlare default or custom HTML with the HTTP status kept | [Origin Error Page Design](./origin-error-page.md) |
| **Edge cache** | single-node OpenResty `proxy_cache`; default static extensions + origin-header/Set-Cookie gates + default Edge TTL (benchmarked to the CF default model) | [Edge Cache Strategy Design](./edge-cache-design.md) |
| **Zone & domain management** | registrable root domains as the management entry, aggregating explicit domains, domain certificates, and reverse proxy routes | [Zone & Domain Resource Design](./zone-design.md) |
| **Cloudflare DNS pointing** | per ZoneDomain, idempotently point a single Cloudflare A record at an edge node IPv4; connection config, groups, member orange-cloud, and async sync; no auto-failover in phase 1 | [Cloudflare DNS Pointing Design](./cloudflare-pointing.md) |
| **Config versioning** | global single active version with preview, release, immutable snapshot history, and second-level one-click rollback | [Agent & Publish Model](./agent-design.md) |
| **WAF protection** | visual DAG rule orchestration, manual/auto/subscription IP groups, GeoIP matching, and PoW CC protection | [WAF Design](./waf-design.md) / [WAF Orchestration Rule Design](./waf-orchestration-design.md) / [WAF Usage Guide](../guide/waf-usage.md) |
| **Intranet penetration** | reverse-penetrate and expose intranet web services via Relay nodes and the OpenFlared client | [Tunnel Design](./tunnel-design.md) / [Tunnel Usage Guide](../guide/tunnel-usage.md) |
| **Pages static hosting** | upload or sync pre-built artifacts from Remote URLs or public GitHub Releases; GitHub latest can be periodically checked and optionally auto-published. Immutable deployments are pulled by edge nodes and served locally by OpenResty, supporting rollback, API proxying, and SPA Fallback | [Pages Static Hosting Design](./pages-design.md) / [Pages Usage Guide](../guide/pages-usage.md) |
| **TLS certificate auto-renewal** | explicitly bind certificates to Zone domains; issue/renew via ACME against Let's Encrypt | [Zone & Domain Resource Design](./zone-design.md) |
| **Multi-node monitoring & observability** | access logs as the single truth for business traffic; Agent reports only details and host readings, Server aggregates uniformly; reconciled with Zone/dashboard | [Observability Transport Model](./observability-transport-model.md) / [Edge Observability & Business Traffic Stats](./observability-design.md) / [Reporting Protocol & Tables](./observability-data-model.md) / [System Architecture](./architecture.md) |
| **Log storage** | access logs and observability time series use the switchable log primary DB (follows the business primary DB or ClickHouse); still writable/queryable with ClickHouse off | [Log Store Decoupling](./logstore.md) |
| **Console bilingual** | zh-CN / en without URL prefixes, `NEXT_LOCALE` cookie precedence, static-export compatible | [Frontend i18n design](../superpowers/specs/2026-07-24-frontend-i18n-design.md) |

---

## Core Product Boundaries and Constraints

When developing and contributing code, **you must strictly follow** these business boundaries and technical constraints; don't bypass them for temporary needs:

### 1. Website Config and Upstream Constraints
* **Single-site domain sharing policy**: one route rule corresponds to one website; the site's multiple domains share rate limit, cache, and reverse-proxy upstream config. Differential per-domain service config within the same rule is not supported.
* **Upstream type mutual exclusion**: the upstream must be one of direct address (`direct`), intranet tunnel (`tunnel`), or Pages static hosting (`pages`); mixing within one rule is not allowed.
* **Direct type restrictions**: a direct upstream can be a single or multiple pure `http://` or `https://` addresses (multi-address only supports plain `scheme://host[:port]`); non-HTTP protocols (TCP/UDP) upstreams are not supported.

### 2. WAF Security Boundaries
* **Allowlist priority**: the allowlist has absolute matching power. Only when an allowlist rule isn't hit do the global and custom blocklist filters trigger in order.
* **GeoIP weak dependency**: geo access resolution fully depends on the node-local MaxMind DB. When GeoIP is abnormal or fails to resolve, the system must auto-ignore geo rules — **never** break IP-group filtering or the reverse-proxy main chain's availability.
* **Runtime data decoupling**: OpenResty interception only reads Agent-synced local JSON, never talking to the Server DB. IP group member sync is decoupled from version release via Checksum differential pull for zero-reload smooth effect.

### 3. Intranet Penetration Boundaries
* **HTTP traffic only**: the tunnel components only support HTTP/HTTPS (based on frp's vhost mechanism for single-port domain-route reuse); standalone TCP/UDP port allocation is not supported yet.
* **Dynamic relay config control**: a Relay node, after connecting to the Server, dynamically pulls and syncs global system config via heartbeats (e.g. whether the embedded FRPS Web UI and its port are enabled), but isn't part of the control plane's immutable config version release system.
* **Tunnel/Node system isolation**: Tunnel clients make outbound connections from the intranet and are independent entities from control-plane-hosted edge Nodes (public nodes), authenticated with the dedicated `tunnel_token`.

### 4. Pages Static Hosting Boundaries
* **Pre-built artifact sources**: a project may stay manual-upload, or configure one Remote URL / public GitHub Release asset source. Remote and fixed tags only support manual ops; only GitHub latest enters scheduled checks and can opt into auto-update. Sources are switchable, but immutable deployments and the current production version don't get lost when editing or deleting a source.
* **Archive and resource limits**: supports `zip`, `tar.gz` / `tgz`, `tar.xz` / `txz`, `tar.bz2` / `tbz2`, `tar`, `7z`. Archive cap controlled by `pages_max_package_size_mb` (default 100 MiB, range 1–2048); expanded single-file and total limits are 4× the package cap with a 100 MiB floor, at most 1,000 regular files. Both Server and Agent validate actual bytes and reject path traversal, symlinks/hard links, and special files.
* **Build and runtime boundaries**: currently no source checkout or build execution from external git repos, and no edge Serverless, dynamic SSR, or preview subdomains. Future repo integration must use a separate `git_repository` Provider with a Server-side isolated build executor, emitting only restricted artifacts into the unified artifact pipeline; the Agent never receives repo credentials, external URLs, or clone/install/build commands.

### 5. System and Version Boundaries
* **Globally single active version**: all nodes pull and consume the same globally active config. Per-node-group differentiated config release isn't performed.
* **Single-tenant architecture**: OpenFlare is for a single team deploying on a trusted internal network. Single-tenant by design; fine-grained multi-user roles or multi-tenant resource isolation aren't supported.
* **External infra dependency**: the Server **must depend on** external Redis (or Valkey) for distributed coordination, the Asynq queue, and system cache. The relational DB is PostgreSQL, or SQLite when `database.enabled` is off. ClickHouse **optional**: when off, access logs and observability time series are handled by the current log primary DB (follows the business primary DB); when on, the「Switch Log Database」task can migrate to ClickHouse. Running without Redis is not supported. See [Log Store Decoupling](./logstore.md).

---

## Repository Structure

OpenFlare has converged to a **single monorepo** (Go module `OpenFlare`). The control-plane Server and edge components (Agent, Relay, OpenFlared) share the repo, organized by Wavelet `internal/apps/` domain modules.

When contributing code, strictly follow this physical layering and directory division:

| Path | Responsibility |
| --- | --- |
| `main.go` | the Server's single entry, delegating to `internal/cmd/` |
| `cmd/agent`, `cmd/relay`, `cmd/flared` | edge component CLI entries (**not** the Server) |
| `internal/` | control-plane and edge runtime implementations |
| `frontend/` | Next.js admin panel; build artifacts embedded into the Go Server |
| `pkg/` | cross-component shared libs (protocol, rendering, GeoIP, etc.) |
| `scripts/` | Swagger generation, install scripts, etc. |
| `docs/` | VitePress docs site and design baseline |
| `manifest/docker/` | per-component Dockerfiles |
| `uploads/`, `data/` | runtime upload dir and static data (`.gitignore`d) |

### 1. Server Layering (`main.go` + `internal/`)

| Directory | Responsibility |
| --- | --- |
| `main.go` | Server startup entry |
| `internal/cmd/` | Cobra subcommands: `api`, `worker`, `scheduler`, `all` (default fused mode) |
| `internal/platform/bootstrap/` | cross-module assembly: task handlers, push domain events, process-level init |
| `internal/router/` | HTTP route registration and global middleware |
| `internal/router/v1/openflare/` | OpenFlare route registrars (`register_*.go`) |
| `internal/apps/openflare/` | OpenFlare control-plane business domains (`routers.go` + `logics.go`) |
| `internal/apps/{admin,user,oauth,upload,cap,...}/` | Wavelet platform capabilities (users, auth, tasks, push, etc.) |
| `internal/apps/openflare/{agent,relay,flared}/` | **Server-side** edge protocol handlers (auth, heartbeat, WS) |
| `internal/model/` | GORM entities / DTOs / no-IO domain rules (`openflare_*.go` + platform models); **no** DB access |
| `internal/infra/persistence/migrator/goose/` | goose SQL migrations (PostgreSQL / SQLite / ClickHouse) |
| `internal/repository/` | data access layer (platform + OpenFlare business CRUD, cache, `logstore` log IO); the **only** persistence entry |
| `internal/infra/task/` | Asynq async tasks (Worker + Scheduler) |
| `internal/infra/config/` | Viper config loading |
| `internal/shared/` | unified API response wrapper (`response/`) |
| `pkg/protocol/` | Relay / Tunnel shared HTTP/WS protocol structures |
| `pkg/render/`, `pkg/geoip/`, `pkg/wsclient/` | OpenResty config rendering, GeoIP, WebSocket client |

**API route prefixes:**

| Prefix | Purpose | Auth |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare admin console API | Session Cookie + optional `X-Access-Token` |
| `/api/v1/agent/*` | Agent node protocol | `X-Agent-Token` |
| `/api/v1/relay/*` | Relay protocol | `X-Agent-Token` |
| `/api/v1/tunnel/*` | Tunnel client protocol | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Wavelet platform admin API | admin Session |

### 2. Agent Modules (`internal/apps/agent/` / `cmd/agent/`)
| `internal/apps/agent/httpclient/`  | Server communication |
| `internal/apps/agent/wsclient/`    | WebSocket client communication |
| `internal/apps/agent/protocol/`    | Agent API protocol types |
| `internal/apps/agent/updater/`     | Agent self-update logic |
| `internal/apps/agent/logging/`     | logging |
| `internal/apps/agent/observability/`| observability (metrics, traces, etc.) |
| `internal/apps/agent/geoipdata/`   | GeoIP data handling |
| `internal/apps/agent/geoipupdate/` | GeoIP data updates |
| `internal/apps/agent/agent/`       | core Agent logic and lifecycle |

### 3. Frontend Layering (`frontend/`)

Based on the Wavelet Next.js scaffold, OpenFlare business UI is organized route-co-located under `app/(main)/`.

| Directory | Responsibility |
| --- | --- |
| `app/` | Next.js App Router; `(main)` console, `(auth)` auth, `(docs)` docs pages |
| `app/(main)/<domain>/` | business pages and in-domain components (route-co-located) |
| `components/` | cross-domain reusable UI (`ui/`, `layout/`, `common/`, etc.) |
| `lib/services/` | API service layer: `core/` base class + `openflare/` business APIs |
| `lib/navigation/` | OpenFlare sidebar nav config (`openflare-nav.ts`) |
| `lib/theme/` | theme parsing and switching |
| `contexts/` | cross-page UI state (user, notifications, etc.) |
| `hooks/`, `lib/hooks/` | reusable React Hooks |
| `public/` | static assets and theme CSS |
| `scripts/` | build helper scripts |
| `proxy.ts` | dev/prod proxy: API rate limit and page auth |

**API conventions**: OpenFlare business APIs uniformly prefix `/api/v1/d/*`, wrapped via `OpenFlareBaseService`; page data fetching uses `@tanstack/react-query`.

### 4. Relay Modules (`internal/apps/relay/` / `cmd/relay/`)

| Module | Responsibility |
| --- | --- |
| `cmd/relay/` | Relay CLI entry and init main |
| `internal/apps/relay/config/` | local config parsing and default init |
| `internal/apps/relay/frps/` | manage frps process lifecycle, ports & Token, monitor runtime |
| `internal/apps/relay/heartbeat/` | periodic HTTP heartbeat, report state, fetch update requests |
| `internal/apps/relay/httpclient/` | generic Server API client helpers |
| `internal/apps/relay/observability/` | collect local host and frps base runtime metrics with pre-aggregation |
| `internal/apps/relay/relay/` | coordinate core lifecycle, init, and cleanup |
| `internal/apps/relay/state/` | local runtime state, error records, persistent cache |
| `internal/apps/relay/updater/` | Relay upgrade check, download/install, restart |
| `internal/apps/relay/wsclient/` | long-lived WebSocket bidirectional channel with the Server |

### 5. OpenFlared (Client) Modules (`internal/apps/flared/` / `cmd/flared/`)

| Module | Responsibility |
| --- | --- |
| `cmd/flared/` | Client CLI entry and init main |
| `internal/apps/flared/config/` | local client config loading and parsing |
| `internal/apps/flared/flared/` | intranet penetration client core scheduling and state management |
| `internal/apps/flared/frpc/` | hot-reload/dynamically generate per-Relay `frpc_{relayNodeID}.toml` and monitor frpc |
| `internal/apps/flared/heartbeat/` | heartbeat communication with the control plane, incl. Token validation |
| `internal/apps/flared/httpclient/` | generic client API communication (`/api/v1/tunnel/*`) |
| `internal/apps/flared/sync/` | incrementally pull latest Tunnel route bindings, generate snapshots, apply |
| `internal/apps/flared/updater/` | client self-update, new-version check, update landing |
| `internal/apps/flared/wsclient/` | WS channel for real-time Server tunnel config change push |

> **Note**: OpenFlared has no standalone `state/` package; version and checksum are persisted by `frpc/manager.go` to `flared-state.json`.

---

## Doc Maintenance Principles

* Product scope or system boundary changes: update this doc ([Product Boundaries](./index.md)).
* Log storage, log-table judgment, or switch-protocol changes: update [Log Store Decoupling](./logstore.md).
* System structure or component division changes: update [System Architecture](./architecture.md).
* Release, sync, rollback, or Agent model changes: update [Agent & Publish Model](./agent-design.md).
* Deployment method changes: update [Deployment Guide](../deployment/deployment.md) and the README.
* Config item changes: update [Configuration Reference](../reference/configuration.md).
