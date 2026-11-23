# Product Boundaries

You will learn: what OpenFlare is, its current stable capabilities, and the core product boundaries and repository structure layout you must follow when developing.

OpenFlare is a modern distributed reverse proxy and edge security gateway platform designed for multi-server clusters and single-team operations.

---

## Project Positioning

OpenFlare suits teams that need to centrally manage multiple OpenFlare Edge proxy nodes, build self-hosted high-availability edge networks, or establish a hardened protective gateway in front of origin servers, with the following positioning:
* **Control/Landing Separation**: The Server control plane never SSHs into proxy nodes; edge nodes actively pull versions and apply them via heartbeats and WebSockets.
* **Zero-Reload Dynamic Configuration Sync**: Built on the Rust OpenFlare Edge gateway, updates take effect via in-memory snapshot atomic swapping and incremental configuration synchronization, eliminating connection jitter caused by traditional gateway reload operations.
* **Integrated Edge Security Gateway**: Combines reverse proxying, automated TLS certificate renewal, WAF protection, Turnstile-like client PoW anti-CC defense, intranet penetration (Tunnel), and Pages static website hosting in a unified control plane.
* **Dual Deployment Architecture**: Operates either as a standalone private edge CDN or as a hardened Origin Shield behind Cloudflare, shielding origin IP addresses while retaining full private audit logs.

**Not this product's positioning**: Multi-tenant public cloud platforms, Kubernetes Ingress Controllers, service meshes, or general-purpose log aggregation systems.

---

## Current Capabilities

| Capability | Description | Detailed Design/Usage |
| --- | --- | --- |
| **Reverse proxy config management** | Website rules (Proxy Route) as the aggregation boundary, supporting multi-domain and multi-upstream load balancing | [Create a Reverse Proxy Config](../guide/proxy-config.md) |
| **Origin error page** | Globally configurable: matching origin/gateway status codes return OpenFlare default or custom HTML with the HTTP status kept | [Origin Error Page Design](./origin-error-page.md) |
| **Edge cache** | OpenFlare Edge in-memory caching: default static extensions + origin-header/Set-Cookie gates + default Edge TTL (benchmarked against the CF default model) | [Edge Cache Strategy Design](./edge-cache-design.md) |
| **Zone & domain management** | Registrable root domains as the management entry, aggregating explicit domains, domain certificates, and reverse proxy routes | [Zone & Domain Resource Design](./zone-design.md) |
| **Cloudflare DNS pointing** | Per ZoneDomain, idempotently point a single Cloudflare A record at an edge node IPv4; connection config, groups, member orange-cloud, and async sync; no auto-failover in phase 1 | [Cloudflare DNS Pointing Design](./cloudflare-pointing.md) |
| **Edge config sync** | Automatic sync on save; nodes apply snapshots atomically while retaining the last healthy snapshot | [OpenFlare Edge node Design](./pingora-edge-design.md) |
| **WAF protection** | Visual DAG rule orchestration, manual/auto/subscription IP groups, GeoIP matching, and PoW CC protection | [WAF Design](./waf-design.md) / [WAF Orchestration Rule Design](./waf-orchestration-design.md) / [WAF Usage Guide](../guide/waf-usage.md) |
| **Intranet penetration** | Reverse-penetrate and expose intranet web services via Relay nodes and the OpenFlared client | [Tunnel Design](./tunnel-design.md) / [Tunnel Usage Guide](../guide/tunnel-usage.md) |
| **Pages static hosting** | Upload or sync pre-built artifacts from Remote URLs or public GitHub Releases; GitHub latest can be periodically checked and optionally auto-published. Immutable deployments are pulled by edge nodes and served locally by OpenFlare Edge, supporting rollback, API proxying, and SPA Fallback | [Pages Static Hosting Design](./pages-design.md) / [Pages Usage Guide](../guide/pages-usage.md) |
| **TLS certificate auto-renewal** | Explicitly bind certificates to Zone domains; issue/renew via ACME against Let's Encrypt | [Zone & Domain Resource Design](./zone-design.md) |
| **Multi-node monitoring & observability** | Access logs as the single truth for business traffic; edge nodes report only details and host readings, Server aggregates uniformly; reconciled with Zone and dashboard | [Observability Transport Model](./observability-transport-model.md) / [Edge Observability & Business Traffic Stats](./observability-design.md) / [Reporting Protocol & Tables](./observability-data-model.md) / [System Architecture](./architecture.md) |
| **Log storage** | Access logs and observability time series use the switchable log primary DB (follows the business primary DB or ClickHouse); still writable/queryable with ClickHouse off | [Log Store Decoupling](./logstore.md) |
| **Console multilingual & i18n** | zh-CN / en / ru without URL prefixes, `NEXT_LOCALE` cookie precedence, static-export compatible | [Runtime Language and Internationalization](./log-language.md) |

---

## Core Product Boundaries and Constraints

When developing and contributing code, **you must strictly follow** these business boundaries and technical constraints; do not bypass them for temporary needs:

### 1. Website Config and Upstream Constraints
* **Single-site domain sharing policy**: One route rule corresponds to one website; the site's multiple domains share rate limit, cache, and reverse-proxy upstream config. Differential per-domain service config within the same rule is not supported.
* **Upstream type mutual exclusion**: The upstream must be one of direct address (`direct`), intranet tunnel (`tunnel`), or Pages static hosting (`pages`); mixing within one rule is not allowed.
* **Direct type restrictions**: A direct upstream can be a single or multiple pure `http://` or `https://` addresses (multi-address only supports plain `scheme://host[:port]`); non-HTTP protocols (TCP/UDP) upstreams are not supported.

### 2. WAF Security Boundaries
* **Allowlist priority**: The allowlist has absolute matching power. Only when an allowlist rule is not hit do global and custom blocklist filters trigger in order.
* **GeoIP weak dependency**: Geographic access resolution fully depends on the node-local MaxMind database. When GeoIP is abnormal or fails to resolve, the system must automatically ignore geographic rules — **never** compromise IP-group filtering or reverse-proxy main chain availability.
* **Runtime data decoupling**: OpenFlare Edge executes rules via in-memory snapshots without communicating with the Server DB. IP group members update independently from site snapshots through checksum-based differential pulls for zero-reload smooth activation.

### 3. Intranet Penetration Boundaries
* **HTTP traffic only**: Tunnel components support only HTTP/HTTPS protocols (leveraging frp virtual host routing to reuse single ports across multiple domains); standalone TCP/UDP port allocation is not supported.
* **Dynamic relay config control**: Relay nodes dynamically pull and synchronize global system configuration (such as embedded FRPS Web UI toggles and ports) via heartbeats after connecting to the Server, governed by a standalone relay configuration protocol.
* **Tunnel/Node system isolation**: Tunnel clients initiate outbound connections from internal networks and remain strictly isolated from control-plane-hosted edge nodes (public nodes), authenticated with dedicated `tunnel_token` credentials.

### 4. Pages Static Hosting Boundaries
* **Pre-built artifact sources**: Projects can maintain manual uploads or configure a single Remote URL / public GitHub Release asset source. Remote and fixed tags support manual operations only; only GitHub latest enters scheduled polling with optional auto-updates. Sources can be modified without losing immutable deployments or the active production release.
* **Archive and resource limits**: Supports `zip`, `tar.gz` / `tgz`, `tar.xz` / `txz`, `tar.bz2` / `tbz2`, `tar`, and `7z`. Package sizes are capped by `pages_max_package_size_mb` (default 100 MiB, range 1–2048); expanded single-file and total size caps are 4× the package limit with a 100 MiB minimum, up to 1,000 regular files. Both Server and edge nodes validate actual bytes and reject path traversals, symlinks/hardlinks, and special files.
* **Build and runtime boundaries**: The system does not check out external Git repositories or execute arbitrary build pipelines, nor does it provide edge Serverless, dynamic SSR, or preview subdomains. Future repository integration must use a standalone `git_repository` Provider with an isolated Server build executor, outputting restricted artifacts into the unified pipeline; edge nodes never receive repository credentials, external URLs, or clone/build commands.

### 5. System and Version Boundaries
* **Site snapshots**: All edge nodes synchronize the same site snapshot, with each node operating under its assigned performance policy.
* **Single-tenant architecture**: OpenFlare is designed for single-team operations on trusted networks. It uses a single-tenant design without fine-grained multi-user roles or multi-tenant resource isolation.
* **External infra dependency**: The Server **requires** external Redis (or Valkey) for distributed coordination, Asynq background workers, and system caching. The relational database is PostgreSQL, or SQLite when `database.enabled` is disabled. ClickHouse is **optional**: when disabled, access logs and observability time series are handled by the log primary database (following the business primary DB); when enabled, data can be migrated via the "Switch Log Database" background task. Running without Redis is not supported. See [Log Store Decoupling](./logstore.md).

---

## Repository Structure

OpenFlare is structured as a **single monorepo** (Go module `github.com/Rain-kl/Wavelet`). The control-plane Server and edge components (Relay, OpenFlared) share the Go codebase, organized by Wavelet `internal/apps/` domain modules, while the edge proxy runtime is implemented in Rust under `edge/pingora/`.

When contributing code, strictly follow this physical layering and directory division:

| Path | Responsibility |
| --- | --- |
| `cmd/server`, `cmd/relay`, `cmd/flared` | CLI entrypoints for Server, Relay, and OpenFlared (Server in `cmd/server/main.go` + `cmd/server/cli/`) |
| `edge/pingora/` | Rust OpenFlare Edge gateway (`openflare-edge`) source and build directory |
| `internal/` | Control-plane and edge runtime implementations |
| `frontend/` | Next.js admin console; build artifacts embedded into the Go Server |
| `pkg/` | Generic cross-cutting libraries (GeoIP, cache, mail, push, utilities, etc.) |
| `scripts/` | Swagger generation, installation, and deployment scripts |
| `docs/` | Fumadocs documentation content and Next.js static site |
| `docker/` | Container Dockerfiles for each component |
| `tests/integration/` | Cross-module integration tests |
| `uploads/`, `data/` | Runtime upload directory and persistent data (`.gitignore`d) |

### 1. Server & Shared Domain Layering (`cmd/server/` + `internal/`)

| Directory | Responsibility |
| --- | --- |
| `cmd/server/` | Server startup entry (`main.go`) and Cobra subcommands under `cli/` (`api`, `worker`, `scheduler`, `all`) |
| `internal/platform/` | Cross-module assembly and lifecycle: `bootstrap/`, `buildinfo/`, `lifecycle/` |
| `internal/router/` | HTTP route registration and global middleware (OpenFlare mounted in `v1/openflare_console.go` and `v1/openflare_edge.go`) |
| `internal/apps/openflare/` | OpenFlare control-plane business domains (`routers.go` + `logics.go`, `tasks/`, `logwriter/`) |
| `internal/apps/{admin,user,oauth,upload,cap,...}/` | Wavelet platform capabilities (users, auth, uploads, tasks, push notifications, etc.) |
| `internal/apps/openflare/{agent,relay,flared}/` | **Server-side** edge protocol handlers (auth, heartbeat, WebSockets) |
| `internal/openflare/` | Shared OpenFlare domain packages: `protocol/`, `edgeconfig/`, `pagesarchive/`, `routeidentity/` |
| `internal/edge/` | Edge runtime shared foundation and component runtimes: `common/`, `relay/`, `flared/` |
| `internal/model/` | GORM entities / DTOs / no-IO domain rules (`openflare_*.go` + platform models); **no** direct DB access |
| `internal/infra/persistence/migrator/goose/` | Goose SQL migrations (PostgreSQL / SQLite / ClickHouse) |
| `internal/repository/` | Data access layer (platform + OpenFlare business CRUD, cache, `logstore` log/analytics I/O); the **only** persistence entrypoint |
| `internal/infra/task/` | Asynq background tasks (Worker + Scheduler) |
| `internal/infra/config/` | Viper configuration loader |
| `internal/shared/` | Unified API response wrappers (`response/`) and HTTP binding helpers (`httpx/`) |
| `pkg/` | Generic utility libraries (`cache/`, `cap/`, `geoip/`, `httppool/`, `i18n/`, `logger/`, `mail/`, `push/`, `trace/`, `util/`, `wsclient/`) |

**API Route Prefixes:**

| Prefix | Purpose | Auth |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare admin console APIs | Session Cookie + optional `X-Access-Token` |
| `/api/v1/agent/*` | Edge node protocol | `X-Agent-Token` |
| `/api/v1/relay/*` | Relay protocol | `X-Agent-Token` |
| `/api/v1/tunnel/*` | Tunnel client protocol | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Wavelet platform admin APIs | Admin Session |

### 2. OpenFlare Edge node

The node runtime is located at `edge/pingora/`. The control plane distributes sites, WAF IP groups, and performance policies via configuration manifests. Nodes validate and apply updates to their local data plane. See [OpenFlare Edge node Design](./pingora-edge-design.md).

### 3. Frontend Layering (`frontend/`)

Based on the Wavelet Next.js scaffold, OpenFlare business UI is organized route-co-located under `app/(main)/`.

| Directory | Responsibility |
| --- | --- |
| `app/` | Next.js App Router; `(main)` console, `(auth)` auth, `(docs)` docs pages |
| `app/(main)/<domain>/` | Business pages and in-domain components (route-co-located) |
| `components/` | Cross-domain reusable UI (`ui/`, `layout/`, `common/`, etc.) |
| `lib/services/` | API service layer: `core/` base class + `openflare/` business APIs |
| `lib/navigation/` | OpenFlare sidebar navigation configuration (`openflare-nav.ts`) |
| `lib/theme/` | Theme parsing and switching |
| `contexts/` | Cross-page UI state (user, notifications, etc.) |
| `hooks/`, `lib/hooks/` | Reusable React Hooks |
| `public/` | Static assets and theme CSS |
| `scripts/` | Build helper scripts |
| `proxy.ts` | Dev/prod proxy: API rate limiting and page authorization |

**API Conventions**: OpenFlare business APIs uniformly prefix `/api/v1/d/*`, wrapped via `OpenFlareBaseService`; page data fetching uses `@tanstack/react-query`.

### 4. Relay Modules (`internal/edge/relay/` / `cmd/relay/`)

| Module | Responsibility |
| --- | --- |
| `cmd/relay/` | Relay CLI entry and main initialization |
| `internal/edge/relay/config/` | Local configuration parsing and default initialization |
| `internal/edge/relay/frps/` | Manages frps process lifecycle, ports, and tokens while monitoring runtime status |
| `internal/edge/relay/heartbeat/` | Periodic HTTP heartbeats, reports status, and retrieves update requests |
| `internal/edge/relay/httpclient/` | Generic Server API client helpers |
| `internal/edge/relay/observability/` | Collects local host and frps runtime metrics with pre-aggregation |
| `internal/edge/relay/relay/` | Coordinates core lifecycle, initialization, and cleanup |
| `internal/edge/relay/state/` | Local runtime state, error records, and persistent cache |

### 5. OpenFlared (Client) Modules (`internal/edge/flared/` / `cmd/flared/`)

| Module | Responsibility |
| --- | --- |
| `cmd/flared/` | Client CLI entry and main initialization |
| `internal/edge/flared/config/` | Local client configuration loading and parsing |
| `internal/edge/flared/flared/` | Intranet penetration client core scheduling and state management |
| `internal/edge/flared/frpc/` | Dynamically generates per-Relay `frpc_{relayNodeID}.toml` and monitors frpc |
| `internal/edge/flared/heartbeat/` | Heartbeat communication with the control plane, including token validation |
| `internal/edge/flared/httpclient/` | Generic client API communication (`/api/v1/tunnel/*`) |
| `internal/edge/flared/sync/` | Incrementally pulls latest Tunnel route bindings, generates snapshots, and applies them |

> **Note**: OpenFlared has no standalone `state/` package; version and checksum are persisted by `frpc/manager.go` to `flared-state.json`.

---

## Doc Maintenance Principles

* Product scope or system boundary changes: update this doc ([Product Boundaries](./index.md)).
* Log storage, log-table judgment, or switch-protocol changes: update [Log Store Decoupling](./logstore.md).
* System structure or component division changes: update [System Architecture](./architecture.md).
* Configuration synchronization and node recovery logic are documented in [OpenFlare Edge node Design](./pingora-edge-design.md).
* Deployment method changes: update [Deployment Guide](../deployment/deployment.md) and the README.
* Config item changes: update [Configuration Reference](../reference/configuration.md).
