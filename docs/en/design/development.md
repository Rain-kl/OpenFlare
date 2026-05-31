# Development Constraints

You will learn: The admission criteria for OpenFlare code modifications, backend/Agent/frontend tiered constraints, data model boundaries, API conventions, database migration requirements, and test delivery baselines.

This document integrates the original development specifications, frontend specifications, and development plans, and serves as the engineering constraints entry point for OpenFlare after `1.0.0`.

## Current Conclusions

* The mainline capabilities of the first to sixth versions have all been completed.
* `1.0.0` is the current official baseline.
* Procedural tasks of completed stages are subject to code, tests, and Git history.
* Priority for new work is given to bug fixes, maintainability improvements, and documentation and test reinforcement.

Current Development Priorities:

1. Stability.
2. Upgrade and rollback link reliability.
3. Document accuracy.
4. Test coverage reinforcement.
5. Small iterations within existing boundaries.

## Change Admission

Before new requirements enter implementation, judge them in the following order:

1. Whether it fits the [Product Boundary](./index.md).
2. Whether it follows the backend, Agent, and frontend constraints in this document.
3. Whether it risks breaking the existing publish, sync, rollback, or upgrade main links.
4. Whether it requires synchronized updates to deployment, configuration, README, or documentation site pages.

If a requirement expands the boundary or introduces new infrastructure, the design documentation must be updated first before starting implementation.

Any changes merged into the official baseline must at least meet:

* Does not break the Agent heartbeat, synchronization, publishing, and rollback main links.
* Does not break the existing OpenResty main configuration hosting model.
* Does not degrade the existing availability of the overview, node details, and access analysis.
* Has tests or joint debugging verification commensurate with the risks.
* Documentation remains consistent with the code.

## Technical Baseline

Server:

* Go 1.25+
* Gin
* GORM
* SQLite / PostgreSQL
* Existing login system

Agent:

* Single binary
* Node-local execution
* Control OpenResty binary via `openresty_path` or default `openresty`
* Docker deployment uses the Agent image with built-in OpenResty, and does not have the Agent control a separate OpenResty container

Frontend:

* Next.js 15 App Router
* React 19
* TypeScript 5
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand only used for lightweight client status
* ESLint + Prettier
* Vitest + Testing Library + Playwright
* pnpm

## Server Layering

| Directory | Responsibility |
| --- | --- |
| `controller/` | Parameter parsing, calling services, returning responses |
| `service/` | Business logic, verification, transaction orchestration, rendering |
| `model/` | Model definition and persistence |
| `router/` | Route registration |
| `middleware/` | Auth, authorization, rate limiting, and other cross-cutting logic |
| `common/` | Configuration, global state, and initialization entry points |
| `utils/` | Pure utility functions and general helpers |

It is forbidden to accumulate business logic in `controller/`, forbidden to implement business flows in `middleware/`, and forbidden to add platform-level abstractions for simple requirements.

## Agent Layering

The Agent maintains its existing module boundaries:

* `config`
* `heartbeat`
* `sync`
* `openresty` / `nginx`
* `state`
* `httpclient`
* `protocol`
* `internal/updater`

Requirements:

* Each module has a single responsibility.
* External command calls are centrally encapsulated.
* State persistence and configuration persistence are separated.

## Frontend Layering

Recommended directories:

```text
app/
components/
features/
lib/
hooks/
store/
types/
styles/
tests/
```

Responsibility constraints:

* `app/`: Routes, layouts, page assembly.
* `features/`: Organize modules by business domains.
* `components/`: Reuse components across features.
* `lib/`: Request client, environment variables, utility functions, constants.
* `store/`: A small amount of cross-page UI state.
* `types/`: Shared type definitions.

Page files are only responsible for obtaining routing parameters, organizing page structures, and calling feature components; they should not handwrite complex API details, complex form verification logic, or maintain a large amount of mutually coupled local states.

## Data Model Specifications

Currently active entities:

* `proxy_routes`
* `origins`
* `config_versions`
* `nodes`
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
* `options`
* `waf_rule_groups`
* `waf_rule_group_bindings`

General constraints:

* No new platform-oriented objects are added unless explicitly required by the design document.
* `origins` only serves as a reusable origin address directory, and the fields are kept lightweight.
* `proxy_routes` uses "site configuration" as the aggregation boundary and must contain a unique `site_name` and a non-empty `domains` list.
* Each domain in `proxy_routes.domains` must be globally unique, and the first item in the list is treated as the primary domain.
* `proxy_routes` continues to allow saving one or more upstream addresses for load balancing, but does not introduce an independent `origin_pool`.
* The legacy `domain` field can only be used as a compatible mirror of `domains[0]`; new code must not continue to use this field as the unique business input.
* If `proxy_routes` is associated with `origins`, it must also save the `origin_url` that can be directly rendered.
* Upstreams uniformly use named `upstream` + keepalive; for a single upstream carrying a base path or query, the original URI should be added back to `proxy_pass`. For multiple upstreams, only pure `scheme://host[:port]` is allowed.
* Rate limits, reverse proxy, and cache configurations currently belong to the site-level `proxy_routes`.
* HTTPS certificate binding must be saved on a per-domain basis through `domain_cert_ids` parallel to `domains`; domains not bound to a certificate must not participate in HTTPS rendering.
* WAF global rule groups are applied to all websites by default, while custom rule groups are bound to site configurations via `waf_rule_group_bindings`; they must be included in the complete configuration version snapshot during publishing.
* `config_versions` must save complete snapshots and rendering results.
* There can only be one activated version globally at a time.
* Rollback is achieved by reactivating older versions.
* `nodes` only retains control plane status and low-frequency summaries.
* Observability data must be associated with nodes and time windows, and snapshots and aggregation results use an append-only model.
* Original access details must have a controlled retention policy.
* `auth_sources` only saves management console third-party login source configurations, currently supporting `github` and `oidc`.
* `external_accounts` is the unique source of binding between third-party accounts and local users; the old `users.github_id` is only used for compatible migration and must not be used as the business input for the new login flow.

## Database Migration

Any modification involving table structures, indexes, column types, sharding rules, or internal persistence metadata must upgrade the database version number in sync.

The database version number is defined in `openflare_server/model`, and it must not rely solely on `AutoMigrate` for implicit upgrades of existing databases.

Every time the database version number is upgraded, an explicit migration method from the previous version to the new version must be added. The migration method must contain validation logic after the upgrade; only when the validation passes can the new database version record be written.

Versions 1 through 7 are treated as the historical initial baseline and no longer keep per-version upgrade files. Starting from v8, database migrations must be placed under `openflare_server/model/migrate` and named after the target version, such as `v16.go`. Each version file registers its migration through `init()`, and the current database version is derived from the highest registered target version. Do not change the semantics of released v8+ migrations merely to reorganize files.

After starting the new package, the database's current version must be checked first, and then upgraded step by step in order to the target version; skipping intermediate upgrade steps to directly write the target version is prohibited.

An empty database initialization can directly establish the current version structure, but the same-version validation must still be executed after the initialization is completed, and the current database version must be persisted.

If the migration or validation fails, the startup process must abort, and the database version record must not be upgraded. Submissions involving database version changes must add corresponding migration tests or equivalent regression tests.

## API and Authentication

The management console and Agent APIs uniformly use JSON. Both success and failure must return a clear `message`:

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

Conventions:

* Agent APIs are uniformly placed under `/api/agent/*`.
* The overview and node details prioritize using dedicated aggregation interfaces.
* Management console mutation APIs uniformly use `POST`; read-only APIs use `GET`.
* The management console continues to reuse existing logins, roles, and Sessions.
* Third-party login uniformly enters through authentication source APIs; authentication source management interfaces must require Root Session.
* `/api/status` can only return the public fields of enabled authentication sources, and must not return the Client Secret.
* When a third-party account is not bound and registration is closed, a process to bind to an existing account should be provided, and users must not be automatically created.
* Official Agent requests uniformly use the node-exclusive `agent_token`.
* The first access can use the global `discovery_token`.
* Agent request headers uniformly use `X-Agent-Token`.

It is forbidden to expose remote shell or arbitrary command execution entries, forbidden to print full Tokens in logs, and forbidden to save main configuration templates that bypass placeholder constraints.

## Publishing and Runtime

The publishing logic must maintain:

* Read all enabled `proxy_routes` during publishing.
* Read OpenResty main configuration parameters, reverse proxy performance parameters, and cache parameters at the same time.
* Generate complete OpenResty configuration.
* Calculate `checksum`.
* Write to `config_versions`.
* Activate the version by switching `is_active`.

Version constraints:

* The version number format is fixed as `YYYYMMDD-NNN`.
* Do not modify historical versions online.
* Do not make differentiated versions grouped by nodes.
* Preview and diff are read-only capabilities and do not generate release records.

The Agent must satisfy:

* Read or generate local `node_id` after startup.
* Periodic heartbeats and synchronization.
* Conventional synchronization prioritizes judging based on the version summary returned by the heartbeat.
* When WS connection upgrade is enabled and the connection is successful, the Agent can receive active version summaries via WS and immediately synchronize; WS failure or disconnection must fall back to HTTP heartbeats.
* Back up old files first when discovering a new version.
* Write main configurations, route configurations, and necessary certificate files.
* Write WAF/PoW runtime configurations, and ensure WAF Lua resources are managed uniformly by the Agent.
* Execute `openresty -t -c <main_config_path>` after writing the new configuration, and then reload; direct startup of OpenResty is allowed when reload finds that it is not running.
* Periodic runtime health checks must not call `openresty -t`, preventing health probes from triggering synchronous upstream domain name resolutions; they should prioritize requesting `/openflare/stub_status` on the local `openresty_observability_port`, using HTTP `200 OK` as the basis for judging that the OpenResty main process and workers are serving.
* If the activation of the new configuration fails, the Agent must first try to restore execution with the target configuration, then roll back to the old configuration and pull up OpenResty again.
* Report warning when OpenResty recovers normally after rollback; if there is no historical main configuration to restore locally, it must be allowed to write the built-in safe fallback configuration and pull up an OpenResty runtime state that only listens to port `80` externally and uniformly returns `503 Service Unavailable` and `OpenFlare: No Valid Configuration`, while retaining the local `stub_status` health check entry. The fallback runtime state must not clear the blocked status of the failed target; the application logs must reflect that the target version failed but the fallback runtime has started. Report failure when there is a historical main configuration but it still cannot recover after rollback.
* Once a target `version + checksum` application fails and rolls back, the Agent must block repeated applications of this target in its local state.
* When the Agent maintains the local MaxMind mmdb, download or refresh failures can only record warnings, and must not block heartbeats, synchronization, configuration application, or OpenResty health checks.

## Frontend Requests, State, and Types

All API requests must be uniformly routed through `lib/api/`:

* Uniformly handle the `success/message/data` response structure.
* Uniformly handle authentication failure, network exceptions, and general error messages.
* Centralize maintenance of resource interfaces and request paths.

State Layering:

* Server state: TanStack Query.
* Page temporary state: Component-internal `useState`.
* Cross-page UI state: Zustand.

Strict TypeScript mode is required; abuse of `any` is prohibited. API responses, form inputs, and business entities must have explicit types.

## Forms, Interaction, Style, and Themes

Forms uniformly use React Hook Form and Zod.

High-risk operations must have double confirmation, show the name of the operation object, and clearly provide success and failure feedback.

Style principles:

* Uniformly use Tailwind CSS and the existing token system.
* Prioritize reusing existing basic components and layout components.
* Maintain consistent visual hierarchy, padding, and semantic colors.

Theme requirements:

* Support `light`, `dark`, and `system` simultaneously.
* User choices must be persisted.
* Try to avoid theme flickering on the first screen.

## Test and Delivery

* Key business logic must have unit tests or equivalent regression tests.
* Agent main link modifications must verify synchronization, application, and rollback.
* Frontend pages must cover at least loading states, empty states, error states, and success feedback.
* When the Go version is adjusted, check `go.mod`, Dockerfile, and CI workflows in sync.

## Subsequent Maintenance

Subsequent planning is no longer maintained in the form of "major version phase documents", but adopts the following methods:

* Product boundary changes: Update [Product Boundary](./index.md).
* Engineering constraint changes: Update this document.
* Deployment and configuration changes: Update [Deployment Guide](../guide/deployment.md), [Configuration Items](../reference/configuration.md), and README.

If explicit new stage goals appear in the future, add dedicated planning documents separately; do not pile completed historical plans back into this document.

The model boundary of the current special topic "Site-level Rules and Configuration Interface Reconstruction" has been integrated into the [Product Boundary](./index.md). When executing, still advance in the order of data models, interfaces, frontend pages, migration tests, and document linkage.
