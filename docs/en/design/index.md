# Product Boundaries & Repository Structure

OpenFlare is a distributed reverse proxy, edge security gateway, and intelligent authoritative DNS platform designed for multi-server clusters and single-team operations.

## Project Positioning

OpenFlare targets teams operating clusters of OpenFlare Edge proxy nodes and OpenFlare DNS servers that want a self-hosted edge network or an origin shield in front of upstream services:

* **Control and data separation**: The Server control plane never SSHs into nodes; edge and DNS nodes pull versioned snapshots via heartbeats and WebSockets.
* **Zero-reload dynamic sync**: `openflare-edge` swaps validated in-memory snapshots atomically without process reloads or dropped connections.
* **Unified security and steering**: One control plane coordinates reverse proxying, quality-aware SmartDNS, multi-provider DNS accounts, automated TLS certificates, WAF rules, client PoW challenges, intranet tunnels, and Pages static hosting.
* **Dual deployment modes**: Runs as a standalone private edge CDN or behind Cloudflare as an Origin Shield.

**Out of scope**: Multi-tenant public clouds, Kubernetes Ingress Controllers, service meshes, or general-purpose log aggregation platforms.

## Current Capabilities

| Capability | Summary | Design & Guide |
| --- | --- | --- |
| **Reverse proxy routing** | Proxy routes (`of_proxy_routes`) aggregate domains, upstreams, rate limits, and caching | [Configure a reverse proxy](../guide/proxy-config.md) |
| **Origin error pages** | Serves built-in or custom HTML on configured error status codes while preserving the HTTP status | [Origin Error Page Design](./origin-error-page.md) |
| **Edge cache** | Node-local TinyUFO memory/disk cache gated by static extensions and origin response headers | [Edge Cache Strategy Design](./edge-cache-design.md) |
| **Zones & domains** | Registrable root domains group explicit and wildcard hostnames with route and cert bindings | [Zone & Domain Resource Design](./zone-design.md) |
| **Multi-provider DNS & SmartDNS** | Manages Cloudflare, Alibaba Cloud DNS, and Tencent Cloud DNSPod accounts, failover groups, and standalone `openflare-dns` servers | [Multi-Provider DNS Pointing & Failover Design](./cloudflare-pointing.md) / [SmartDNS System Architecture Design](./smartdns-design.md) |
| **Network intelligence & quality** | Combines multi-source GeoIP/ASN/BGP intelligence, passive `TCP_INFO`, isolated probes, an adaptive matrix, and a five-tier predictor | [Network Intelligence and Link Quality Architecture](./network-quality-design.md) |
| **Edge config sync** | Compiles versioned snapshots on save; nodes validate and swap atomically while keeping the last healthy snapshot | [OpenFlare Edge Architecture Design](./pingora-edge-design.md) |
| **WAF & PoW protection** | Visual DAG rules, OWASP CRS inspection, manual/auto/subscription IP groups, GeoIP matching, and browser PoW | [WAF Orchestration Rule Design](./waf-orchestration-design.md) / [PoW Protection Design](./pow-design.md) |
| **Intranet tunnels** | Native Rust relay (`openflare-relay`) and connector (`openflared`) with zone-diverse HA and L4/L7 forwarding | [Tunnel & Intranet Penetration Design](./tunnel-design.md) |
| **Pages static hosting** | Imports prebuilt archives via uploads, Remote URLs, or GitHub Releases and serves them from edge nodes | [Pages Static Hosting Design](./pages-design.md) |
| **Observability & log storage** | Treats access logs as the single traffic truth and routes telemetry through `logstore` (PostgreSQL / SQLite / ClickHouse) | [Edge Observability and Traffic Analytics Architecture](./observability-design.md) / [Log Store Decoupling](./logstore.md) |
| **Runtime i18n** | Console supports `zh-CN` / `en` / `ru` (`NEXT_LOCALE` precedence); operational logs stay in English | [Runtime Language and Internationalization](./log-language.md) |

## Core Product Boundaries & Constraints

Contributors must adhere to these architectural boundaries:

### 1. Site Routing & Upstream Constraints
* **Single-site policy sharing**: One route binds multiple hostnames that share upstream, rate-limit, and cache policies; per-hostname overrides within one route are not supported.
* **Mutually exclusive upstreams**: A route targets exactly one upstream mode: `direct`, `tunnel`, or `pages`.
* **Direct upstream restrictions**: Direct upstreams accept only `http://` or `https://` targets; multi-origin entries allow plain `scheme://host[:port]` values without paths.

### 2. WAF Security Boundaries
* **Allowlist precedence**: Global request allowlists bypass WAF DAG evaluation and body inspection; remaining requests run global rules before ordered route-bound rules.
* **Weak GeoIP dependency**: Geographic nodes read local MaxMind MMDB files and evaluate as non-matching if missing or unreadable without interrupting IP filtering or proxying.
* **Control DB decoupling**: Edge nodes evaluate compiled in-memory snapshots and never query the control-plane database; IP groups refresh independently by checksum.

### 3. Tunnel Boundaries
* **Dual L4 & L7 protocols**: L7 HTTP/WebSocket traffic routes from OpenFlare Edge to Relay Gateways; L4 TCP/UDP ports listen directly on relays and forward over embedded `rathole` tunnels.
* **Domain and auth isolation**: Relay pools, relays, tunnels, connectors, and L4 services reside under `/tunnels`, authenticate via `oftok1_` bootstrap tokens, `X-Relay-Token`, and `X-Tunnel-Token`, and release on `tunnel-v*`.
* **Origin SSRF guard**: `openflared` validates resolved target IPs against allow/block CIDRs before connecting (blocking `169.254.169.254/32` by default).

### 4. Pages Static Hosting Boundaries
* **Prebuilt artifact ingestion**: Accepts manual uploads or a Remote URL / public GitHub Release source. Editing or removing a source leaves immutable deployments and active releases intact.
* **Archive & size limits**: Supports `zip`, `tar.gz`/`tgz`, `tar.xz`/`txz`, `tar.bz2`/`tbz2`, `tar`, and `7z`. Archive size is capped by `pages_max_package_size_mb` (default `100 MiB`, range `1–2048`); extracted file/total limits are 4x the archive limit (minimum `100 MiB`) across up to `1,000` regular files.
* **No remote source builds**: OpenFlare does not clone Git repositories or run build commands; edge nodes consume validated artifacts only.

### 5. System & Infrastructure Boundaries
* **Shared site snapshot**: All edge nodes synchronize the same site snapshot and operate under their assigned performance policy.
* **Single-tenant model**: Built for single-team operations without multi-tenant resource isolation or RBAC roles.
* **External dependencies**: Server requires Redis (or Valkey) for Asynq queues and cache invalidation; primary storage uses PostgreSQL (or SQLite when `database.enabled=false`); ClickHouse is optional (see [Log Store Decoupling](./logstore.md)).

## Repository Structure

OpenFlare uses a single monorepo (Go module `github.com/Rain-kl/Wavelet`, Rust edge gateway under `edge/`, Rust DNS server under `dns/`, and Rust tunnel workspace under `tunnel/`):

| Path | Responsibility |
| --- | --- |
| `cmd/server/`, `cmd/openflare/` | Control-plane Server entrypoint (`main.go` + `cli/`) and `openflare` operations CLI |
| `edge/` | Rust OpenFlare Edge gateway source and smoke tests |
| `dns/` | Rust OpenFlare DNS authoritative server source |
| `tunnel/` | Rust native tunnel workspace (`common/`, `relay/`, `flared/`) |
| `shared/` | Shared Rust isolated probing (`probe/`) and static templates (`edgeassets/`) |
| `internal/` | Control-plane business domains, repositories, and shared wire packages |
| `frontend/` | Next.js admin console embedded into the Go Server binary |
| `pkg/` | Cross-cutting libraries (`cache/`, `cap/`, `geoip/`, `httppool/`, `i18n/`, `logger/`, `mail/`, `netintel/`, `push/`, `trace/`, `util/`) |
| `scripts/` | Swagger generation and host install/update/uninstall scripts |
| `docs/` | Fumadocs documentation site and design specs |
| `docker/` | Component Dockerfiles and container entrypoints |
| `tests/integration/` | Cross-module integration tests |

### Server Control Plane Layering (`cmd/server/` + `internal/`)

| Directory | Responsibility |
| --- | --- |
| `internal/platform/` | Cross-module wiring and lifecycle (`bootstrap/`, `buildinfo/`, `lifecycle/`) |
| `internal/router/` | HTTP routes and middleware (`v1/openflare_*.go`) |
| `internal/apps/openflare/` | OpenFlare domains (`agent/`, `cloudflare/`, `dnsaccount/`, `netintel/`, `netquality/`, `node/`, `observability/`, `pages/`, `pow/`, `proxy_route/`, `smartdns/`, `tunnel/`, `waf/`, `zone/`, etc.) |
| `internal/openflare/` | Shared OpenFlare packages: `dnsprovider/`, `edgeconfig/`, `pagesarchive/`, `protocol/`, `routeidentity/` |
| `internal/model/` | GORM entities, config key constants, and pure no-IO rules |
| `internal/infra/persistence/migrator/goose/` | Goose SQL migrations for PostgreSQL, SQLite, and ClickHouse |
| `internal/repository/` | Exclusive persistence boundary (CRUD, cache, and `logstore` backends) |
| `internal/infra/task/`, `internal/infra/config/` | Asynq task definitions and Viper config loader |
| `internal/shared/` | API response envelopes (`response/`) and HTTP binding helpers (`httpx/`) |

**API Route Prefixes**:

| Prefix | Purpose | Authentication |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare console APIs | Session Cookie + optional `X-Access-Token` |
| `/api/v1/agent/*` | OpenFlare Edge protocol | `X-Agent-Token` |
| `/api/v1/dns-agent/*` | OpenFlare DNS protocol | `X-DNS-Discovery-Token` / `X-DNS-Token` |
| `/api/v1/relay/*` | Relay protocol | `X-Relay-Bootstrap-Token` / `X-Relay-Token` |
| `/api/v1/tunnel/*` | OpenFlared connector protocol | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Platform admin APIs | Admin Session |
