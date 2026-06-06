# System Architecture

You will learn: The overall architecture of OpenFlare, the boundaries of responsibilities for Server, Agent, OpenResty, and Admin Frontend, and the request flow of a configuration publication from the admin dashboard to activation on a node.

OpenFlare consists of the Server, the Agent, the node-local OpenResty, and the Admin Frontend. The Server is the control plane, the Agent is the only controlled entry point on the node side, and OpenResty serves as the actual data plane. In intranet penetration scenarios, the Relay (frps manager) and OpenFlared (frpc manager) extend the data plane traffic path.

### Standard Reverse Proxy Traffic Path

```text
Browser
  |
  | Management UI / API
  v
OpenFlare Server (Gin + GORM + SQLite/PostgreSQL)
  |
  | Agent API / heartbeat / config pull
  v
OpenFlare Agent
  |
  | write config / openresty -t / reload / rollback
  v
OpenResty binary
  |
  | reverse proxy
  v
Origin
```

### Intranet Penetration Traffic Path

```text
Browser
  |
  | HTTPS request
  v
OpenResty (Agent, TLS/WAF)         <-- TunnelRelay Node
  |
  | proxy_pass http://localhost:vhost_port (Host header preserved)
  v
OpenFlareRelay (frps)              <-- TunnelRelay Node, co-located with Agent
  |
  | frp tunnel protocol (HTTP Vhost routing by Host header)
  v
OpenFlared (frpc)                  <-- Intranet Server
  |
  | HTTP/HTTPS forward
  v
Internal Service (192.168.x.x)
```

## Component Responsibilities

| Component | Responsibility |
| --- | --- |
| Server | Admin UI, Admin API, Agent/Relay/Client API, configuration rendering, version publishing, data storage, and aggregated queries. |
| Agent | Registration, heartbeats, synchronization, file writing, validation, reload, rollback on failure, self-updating, and light metrics collection. |
| OpenResty | Receives real traffic, executing WAF, PoW, authentication, and reverse proxying according to the configuration rendered by OpenFlare. |
| OpenFlareRelay | Manages the lifecycle of the frps process, providing tunnel relay services and receiving frps configurations via heartbeat. |
| OpenFlared | Manages frpc processes (can be multiple), connecting to the Relay and forwarding traffic to intranet services. |
| Frontend | Manages pages for website configs, WAF, origins, certificates, nodes, tunnels, versions, users, settings, and observability. |

## Server

`openflare-server` is the single-control-plane monolith:

* Gin provides the HTTP services.
* GORM accesses SQLite or PostgreSQL.
* The existing login system provides Admin Session management.
* Authentication sources support GitHub OAuth and standard OIDC logins with external account binding.
* The Go Server hosts the `openflare-server/web` static build assets.

The Server does not directly SSH to nodes, nor does it modify node files online. It only stores control plane state, generates complete configuration versions, and lets nodes actively pull them via the Agent API.

## Agent

`openflare-agent` is a Go monolithic application:

* Runs as a single binary on the node side.
* Reads or generates local node information on startup.
* Performs periodic heartbeat check-ins to report status and retrieve active version summaries.
* Upon discovering a new version, it pulls the configuration, backs up old files, writes new files, validates them, and reloads.
* Automatically rolls back to restore operations if the application fails.
* Maintains the local WAF GeoIP mmdb, writing the built-in library on startup and updating it periodically based on configuration.

The Agent executes validation, reload, startup, and restart uniformly via the path specified in `openresty_path`; if unconfigured, it defaults to calling `openresty`. During Docker deployments, the Agent image packages OpenResty and follows the same execution control logic.

The node IP is maintained by default through Agent registration and heartbeat reporting; if the administrator locks the node IP, the Server only updates running status, versions, and observability fields, and no longer accepts reports from the Agent to override the locked IP.

## Frontend

`openflare-server/web` is the official Next.js-based frontend:

* Next.js 15 App Router.
* React 19.
* TypeScript.
* Tailwind CSS.
* TanStack Query for server-side state.

The frontend uses static export mode (`output: 'export'`), which is then hosted by the Go Server using `embed.FS`. All API requests must go through `lib/api/` and process the `success/message/data` response structure.

The Server integrates the following security features:
* CORS middleware: Cross-Origin Resource Sharing protection.
* Rate limiting: Global and key API endpoint throttling.
* Session management: Cookie/Redis-based session storage.

## Data & Request Flow

### Management Request Flow

```text
Browser -> Frontend -> /api/* -> controller -> service -> model -> database
```

Admin mutation APIs use `POST`, while read-only APIs use `GET`. Both success and failure responses return a clear `message`.

### Agent Sync Flow

```text
Agent HTTP heartbeat -> Server returns active version summary
Agent detects new version -> Pulls complete configuration details
Agent writes main configuration / route configurations / certificates / Lua resources / WAF runtimes
Agent runs OpenResty validation (openresty -t) and reload
Agent reports application result
```

### Relay Sync Flow

The Relay (OpenFlareRelay process) runs on the TunnelRelay node and shares the same `agent_token` with the Agent:

```text
Relay HTTP heartbeat -> Server returns frps base configuration (bindPort, vhostHTTPPort, auth_token)
Relay generates frps.toml and starts or updates the frps process
Relay periodically reports frps health status and connection statistics
Relay attempts WebSocket upgrade for real-time configuration pushes
```

frps configurations are relatively static (ports, auth token), dispatched via heartbeats, and **not included in the versioned publishing flow**. The Relay must monitor the frps process and auto-recover it on failures. Authentication: `X-Agent-Token` + API path prefix `/api/relay/*`, distinguished by Server via `node_type = tunnel_relay`.

### OpenFlared Sync Flow

OpenFlared (client) runs inside the intranet server, using independent `tunnel_token` authentication:

```text
Client HTTP heartbeat -> Server returns tunnel configuration version summary (version, checksum)
Client detects new version -> Pulls complete tunnel route configuration (relay list + frpc proxy definitions)
Client generates independent frpc.toml configuration files for each Relay
Client starts a new frpc process for new Relays, or hot-reloads (frpc reload) existing ones
Client reports application results (success/failure details)
```

OpenFlared communicates with the Server via `/api/flared/*` using the `X-Tunnel-Token` header. Tunnel route configurations are versioned along with the publishing flow, ensuring all configuration changes are consistently published to both Agents and Clients via a single version number.

**WebSocket Upgrade Flow** (Optional, controlled via `AgentWebsocketUpgradeEnabled`):

When WebSocket upgrade is enabled:
1. The Agent retrieves run configurations and settings via HTTP heartbeat.
2. The Agent attempts to upgrade the connection to `GET /api/agent/ws` (WebSocket).
3. Once the WS connection is established, periodic state reporting and real-time commands are carried over the WebSocket pipeline, minimizing latency.
4. When the Server publishes or activates a version, it immediately broadcasts the active version summary to connected Agents, triggering the sync flow instantly.
5. If the WebSocket disconnects or fails to establish, the Agent automatically falls back to HTTP heartbeats, ensuring high availability.

Through the `OpenRestyWebsocketEnabled` option, WebSocket reverse proxy support can be enabled or disabled at the OpenResty layer.

### Reverse Proxy Flow

```text
Client -> OpenResty server block -> WAF Lua -> named upstream -> Origin
```

Website configurations are the boundaries of reverse proxy aggregation. A single website configuration can bind multiple domains, sharing site-level rate limiting, reverse proxy, and cache settings.

WAF executes in the OpenResty `access_by_lua_file` phase. Rules originate from the `waf_config.json` carried in the currently active version; global rule groups take effect by default, and websites can overlay custom rule groups. `waf_config.json` only stores rule group references and IP group IDs; IP group members are synchronized independently by the Agent into `waf_ip_groups.json`, and the OpenResty Lua engine merges and evaluates them by reference ID.

WAF IP groups are managed by the Server. Manual IP groups store IP/CIDR lists directly; auto IP groups are evaluated by Server cron jobs reading request logs and applying Expr boolean rules; subscription IP groups are fetched by Server cron jobs from remote text or JSON sources. The Agent reports local IP group checksums in heartbeats, and the Server only returns mismatched IP groups. When an IP group is updated on the Server, a broadcast is sent via WebSocket to push changes, and the OpenResty Lua reads the local JSON file directly without querying the DB, request logs, or remote subscription sources.

## Core Objects

Current valid entities include:

* `proxy_routes`
* `origins`
* `config_versions`
* `nodes`
* `tunnels`
* `auth_sources`
* `external_accounts`
* `node_system_profiles`
* `apply_logs`
* `tls_certificates`
* `managed_domains`
* `node_request_reports`
* `node_access_logs`
* `node_metric_snapshots`
* `traffic_analytics_rollups`
* `node_health_events`
* `waf_rule_groups`
* `waf_ip_groups`
* `waf_rule_group_bindings`
* `acme_accounts`
* `dns_accounts`
* `geoip_update_configs`

## Key Design Decisions

| Decision | Rationale |
| --- | --- |
| Full Config Versioning instead of Patches | Provides stable, verifiable boundaries for previewing, activating, history, and rollbacks. |
| Pull Model (Agent-driven) | Server does not need SSH keys or inbound command ports, preventing control channel hijacking. Supports HTTP and WebSocket. |
| Global Single Active Version | Reduces MVP complexity, ensuring all nodes are uniform by default. Supports previews, version history, and one-click rollback. |
| Website Multi-Domain Aggregation | Enables sharing site-level policies across domains while supporting per-domain certificate binding. |
| Server-side Observability Aggregation | Prevents UI-side temporary statistical calculations from producing inconsistent data metrics. |
| Intranet Penetration based on frp | Reuses a mature tunnel protocol rather than custom implementations to minimize stability risks. frps Vhost routing aligns naturally with HTTP. |
| Independent Binary for Relay/Client | Separation of concerns: Relay manages frps, Client manages frpc, allowing independent updates and deployments. |
| Tunnel decoupled from Node system | Tunnel clients run internally, using completely different registration and authentication flows compared to edge nodes. |

## Recommended Reading for Contributors

Before modifying architectural code, please read:

1. [Product Boundaries](./index.md)
2. [Agent & Publish Model](./agent-design.md)
3. [Development Constraints](../../guideline/Constraints.md)
4. [Repository Structure](./repository.md)
