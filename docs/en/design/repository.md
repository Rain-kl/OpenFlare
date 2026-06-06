# Repository Structure

You will learn: The responsibilities of Server, Agent, Frontend, scripts, and documentation folders in the OpenFlare repository, and where to place logic when contributing code.

| Path | Responsibility |
| --- | --- |
| `openflare-server` | Gin + GORM + SQLite/PostgreSQL single monolithic control plane |
| `openflare-server/web` | Next.js 15 App Router Admin Frontend, hosted by Go Server |
| `openflare-agent` | Go monolithic Agent running on the node side |
| `openflare-relay` | Tunnel relay daemon running on public edges, managing frps processes |
| `openflared` | Tunnel client running on intranet servers, managing frpc processes |
| `scripts` | System helper scripts for installation, self-updating, etc. |
| `docs` | VitePress documentation website, design baselines, specifications, and configurations |
| `docs/en` | English version of documentation |

## Server Layering

| Folder | Responsibility |
| --- | --- |
| `controller/` | Parameter parsing, service calling, and returning responses |
| `service/` | Business logic, validations, transaction orchestration, and configuration rendering |
| `model/` | Model definitions, database versioning, and migrations |
| `router/` | Route registration |
| `middleware/` | Cross-cutting concerns like authentication, authorization, rate limiting, CORS, and Turnstile |
| `common/` | Configurations, global states, and initialization entrypoints |
| `utils/` | Pure utility functions and general helpers |
| `job/` | Periodic cron tasks (such as SSL certificate auto-renewals) |
| `upload/` | File upload handlers |
| `docs/` | API documentation (Swagger) |
| `data/` | Static data (such as GeoIP databases) |

## Agent Modules

| Module | Responsibility |
| --- | --- |
| `config/` | Configuration loading and default values |
| `heartbeat/` | Heartbeat check-in and configuration version evaluation |
| `sync/` | Configuration fetching and application orchestration |
| `nginx/` | OpenResty file writing, validation, reloads, startup, and rollbacks |
| `state/` | Local states and buffers for metric reporting |
| `httpclient/` | Server HTTP API communication |
| `wsclient/` | WebSocket client communication |
| `protocol/` | Agent API protocol types and structures |
| `updater/` | Agent self-updating logic |
| `logging/` | Logging processing |
| `observability/` | Observability (metrics, tracing, etc.) |
| `geoipdata/` | GeoIP database handling |
| `geoipupdate/` | GeoIP database updates |
| `agent/` | Core Agent bootstrap and lifecycle orchestration |

## Frontend Layering

| Folder | Responsibility |
| --- | --- |
| `app/` | Next.js App Router routes, layouts, and page assemblies |
| `features/` | Feature modules organized by business domains |
| `components/` | Reusable UI components shared across features |
| `lib/` | API clients, environment configurations, utility functions, and constants |
| `store/` | Lightweight cross-page UI state management |
| `types/` | Shared TypeScript type definitions |
| `styles/` | Global stylesheets |
| `tests/` | Frontend unit and integration tests (Vitest, Playwright) |
| `scripts/` | Build and deployment scripts |
| `public/` | Static assets |

## Relay Modules

| Module | Responsibility |
| --- | --- |
| `cmd/` | CLI startup entrypoint and main bootstrap functions |
| `internal/config/` | Local configurations parsing and defaults initialization |
| `internal/frps/` | Manages the lifecycle of the frps process, monitoring its status |
| `internal/heartbeat/` | Periodic HTTP heartbeat, status reporting, and update retrievals |
| `internal/httpclient/` | General API client for calling the Server |
| `internal/observability/` | Host and frps metrics collection and pre-aggregation |
| `internal/relay/` | Coordinates the core Relay lifecycle, setup, and cleanup |
| `internal/state/` | Local runtime states, error logs, and persistent caches |
| `internal/updater/` | Relay update check, download installation, and restarts |
| `internal/wsclient/` | Bi-directional real-time WebSocket connection to the Server |

## OpenFlared (Client) Modules

| Module | Responsibility |
| --- | --- |
| `cmd/` | CLI startup entrypoint and main bootstrap functions |
| `internal/config/` | Local client configurations loading and parsing |
| `internal/flared/` | Core client scheduling and tunnel lifecycle orchestration |
| `internal/frpc/` | Dynamically generates `frpc.toml` configs for multiple Relays and monitors frpc processes |
| `internal/heartbeat/` | Heartbeat communications with control planes, including token checks |
| `internal/httpclient/` | General API client for Server communication |
| `internal/sync/` | Incrementally pulls latest Tunnel route bindings, generates snapshots, and applies them |
| `internal/updater/` | Client self-update, new version check, and upgrade installation |
| `internal/wsclient/` | Bi-directional WebSocket client for real-time tunnel configuration pushes |
