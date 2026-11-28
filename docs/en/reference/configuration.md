# Configuration

Supported configuration sources, fields, environment variables, and defaults across OpenFlare Server, frontend build, `openflare-edge`, `openflare-dns`, `openflare-relay`, and `openflared`.

---

## Config Sources

### 1. Server Config Sources
- **Config file**: reads `./config.yaml` at startup (override via `CONFIG_PATH`).
- **Env vars**: every YAML field can be overridden via an `UPPER_SNAKE_CASE` env var (higher precedence than YAML).
- **System runtime config**: stored in `w_system_configs` and dynamically hot-reloaded across workers and edge daemons.

### 2. Data-Plane & Edge Daemon Config Sources
- **CLI flags & env vars**: `openflare-edge`, `openflare-dns`, `openflare-relay`, and `openflared` are Rust binaries configured exclusively through CLI flags and env vars (no JSON/YAML main config files).
- **State directories**: runtime identities, snapshots, and readiness markers are managed inside each component's `--state-dir` / `--cache-dir`.

---

## Config and State File Locations

| Component | Default Location | Notes |
| --- | --- | --- |
| Server config file | `./config.yaml` | Override via `CONFIG_PATH` |
| Server SQLite DB | `openflare.db` | Override via `database.sqlite_path` / `SQLITE_PATH` |
| Edge state dir | `/var/lib/openflare-edge` | Set via `--state-dir` / `OPENFLARE_EDGE_STATE_DIR`; stores `identity.json`, `snapshot.json`, `bootstrap-token`, `ready.json` |
| SmartDNS state dir | `/var/lib/openflare-dns` (container) / `data/smartdns` | Set via `--cache-dir` / `OPENFLARE_DNS_CACHE_DIR`; stores `identity.json`, `last-snapshot.bin`, `ready.json` |
| SmartDNS GeoIP dir | `/var/lib/openflare-dns/geoip` (container) / `data/netintel` | Set via `--geoip-dir` / `OPENFLARE_DNS_GEOIP_DIR` |
| Relay state dir | `/var/lib/openflare-relay` | Set via `--state-dir` / `OPENFLARE_RELAY_STATE_DIR`; stores `relay_identity.json` |
| OpenFlared state dir | `/var/lib/openflared` | Set via `--state-dir` / `OPENFLARE_TUNNEL_STATE_DIR`; stores `connector-id`, `connector_identity.json`, `bootstrap-token` |

---

## Server CLI Args

```bash
# Specify a config file when starting Server
CONFIG_PATH=/path/to/custom-config.yaml ./bin/openflare-server all
```

Supported subcommands:
- `all`: runs API + Worker + Scheduler in one process (default).
- `api`: runs control-plane HTTP/WebSocket API only.
- `worker`: runs Asynq background worker only.
- `scheduler`: runs cron scheduler only.
- `reset-passwd`: resets an account password (`--user`, `--password`).
- `rotate-credentials`: dry-runs (default) or transactionally re-encrypts (`--apply`) DB credentials before changing `app.session_secret`.

---

## Server Env Vars vs Config File

Base settings live in `config.yaml` and accept `UPPER_SNAKE_CASE` env-var overrides.

### 1. App & Security Config (`enable_webshell` / `app:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `enable_webshell` | `ENABLE_WEBSHELL` | Allow Server to dispatch remote shell commands to edge nodes (startup-only) | `false` |
| `app.app_name` | `APP_NAME` | Application identifier | `openflare` |
| `app.env` | `APP_ENV` | Runtime mode (`development` / `testing` / `production`) | `production` |
| `app.addr` | `APP_ADDR` | HTTP bind address | `:3000` |
| `app.node_id` | `APP_NODE_ID` | Snowflake node ID (0-1023); must be unique per replica | `1` |
| `app.api_prefix` | `APP_API_PREFIX` | Admin & API route prefix | `/api` |
| `app.graceful_shutdown_timeout` | `APP_GRACEFUL_SHUTDOWN_TIMEOUT` | Shutdown wait timeout (seconds) | `30` |
| `app.session_cookie_name` | `APP_SESSION_COOKIE_NAME` | Session cookie name | `openflare_session_id` |
| `app.session_secret` | `APP_SESSION_SECRET` | Session signing & DB secret encryption key (**production requires long random key; MCP needs >= 32 bytes**) | none (random) |
| `app.session_domain` | `APP_SESSION_DOMAIN` | Cookie scope domain | empty |
| `app.session_age` | `APP_SESSION_AGE` | Browser session lifetime (seconds) | `86400` (24h) |
| `app.session_http_only` | `APP_SESSION_HTTP_ONLY` | Enable cookie `HttpOnly` flag | `false` |
| `app.session_secure` | `APP_SESSION_SECURE` | Enable cookie `Secure` flag (HTTPS) | `false` |

### 2. Relational DB Config (`database:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `database.enabled` | `DB_ENABLED` | Enable PostgreSQL (`false` uses SQLite) | `true` |
| `database.sqlite_path` | `SQLITE_PATH` | SQLite file path | `openflare.db` |
| `database.host` | `DB_HOST` | PostgreSQL host | `127.0.0.1` |
| `database.port` | `DB_PORT` | PostgreSQL port | `5432` |
| `database.username` | `DB_USERNAME` | PostgreSQL username | `openflare` |
| `database.password` | `DB_PASSWORD` | PostgreSQL password | `replace-with-strong-password` |
| `database.database` | `DB_NAME` | PostgreSQL database name | `openflare` |
| `database.max_idle_conn` | `DB_MAX_IDLE_CONN` | Max idle pool connections | `16` |
| `database.max_open_conn` | `DB_MAX_OPEN_CONN` | Max open pool connections | `128` |
| `database.conn_max_lifetime` | - | Max connection lifetime (seconds) | `1800` |
| `database.conn_max_idle_time` | - | Max idle time (seconds) | `600` |
| `database.log_level` | `DB_LOG_LEVEL` | GORM SQL log level (`info` / `warn` / `error` / `silent`) | `info` |
| `database.ssl_mode` | `DB_SSL_MODE` | PostgreSQL SSL mode | `disable` |
| `database.time_zone` | `DB_TIMEZONE` | Session timezone | `UTC` |
| `database.application_name` | - | DB application identifier | `openflare-server` |
| `database.search_path` | - | PostgreSQL schema search path | `public` |
| `database.prefer_simple_protocol` | - | Prefer simple query protocol | `false` |
| `database.statement_cache_capacity`| - | Prepared statement cache capacity | `256` |
| `database.default_query_exec_mode` | - | Query execution mode | `cache_statement` |
| `database.slow_threshold` | - | Slow query threshold | `200ms` |

SQLite file DBs automatically enable WAL mode on startup (`memory` retains its mode); ensure directory write permission for `-wal`/`-shm` files and use the admin database export endpoint rather than copying live `.db` files directly.

### 3. Redis Config (`redis:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `redis.enabled` | `REDIS_ENABLED` | Enable Redis (required for async queues and sync) | `true` |
| `redis.addrs` | `REDIS_ADDR` | Server or cluster addresses (env sets single address) | `["127.0.0.1:6379"]` |
| `redis.username` | `REDIS_USERNAME` | Redis username | empty |
| `redis.password` | `REDIS_PASSWORD` | Redis password | empty |
| `redis.db` | `REDIS_DB` | Redis DB index | `0` |
| `redis.cluster_mode` | - | Enable Redis cluster mode | `false` |
| `redis.master_name` | - | Sentinel master name | empty |
| `redis.key_prefix` | `REDIS_KEY_PREFIX` | Key prefix | `openflare:` |
| `redis.pool_size` | `REDIS_POOL_SIZE` | Pool size | `100` |
| `redis.min_idle_conn` | - | Min idle connections | `10` |
| `redis.dial_timeout` | - | Dial timeout (seconds) | `5` |
| `redis.read_timeout` | - | Read timeout (seconds) | `3` |
| `redis.write_timeout` | - | Write timeout (seconds) | `3` |
| `redis.max_retries` | - | Max command retries | `3` |
| `redis.pool_timeout` | - | Pool wait timeout (seconds) | `4` |
| `redis.conn_max_idle_time` | - | Max idle time (seconds) | `300` |
| `redis.maint_notifications` | `REDIS_MAINT_NOTIFICATIONS` | Redis maintenance notifications handshake; keep off if compatibility is uncertain | `false` |

### 4. ClickHouse Config (`clickhouse:`)

> **Note**: Client settings below connect Server to ClickHouse. For small-host ClickHouse tuning, mount `./config/clickhouse/performance.xml` to `/etc/clickhouse-server/config.d/performance.xml` (see [Start the Server](../deployment/server.md)).

| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `clickhouse.enabled` | `CLICKHOUSE_ENABLED` | Enable ClickHouse analytics/metrics backend (auto-enabled by `CLICKHOUSE_HOST`) | `false` |
| `clickhouse.hosts` | `CLICKHOUSE_HOST` | Cluster addresses (env sets single address) | `["127.0.0.1:9000"]` |
| `clickhouse.username` | `CLICKHOUSE_USERNAME` | Username | `default` |
| `clickhouse.password` | `CLICKHOUSE_PASSWORD` | Password | `replace-with-clickhouse-password` |
| `clickhouse.database` | `CLICKHOUSE_NAME` | Database name | `openflare` |
| `clickhouse.max_idle_conn` | - | Idle pool connections | `8` |
| `clickhouse.max_open_conn` | - | Max open connections | `16` |
| `clickhouse.conn_max_lifetime` | - | Max connection lifetime (seconds) | `3600` |
| `clickhouse.dial_timeout` | - | Dial timeout (seconds) | `5` |
| `clickhouse.block_buffer_size` | - | Native-protocol block buffer rows | `32` |

### 5. System Log Config (`log:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `log.level` | `LOG_LEVEL` | Log level (`debug` / `info` / `warn` / `error` / `fatal`) | `info` |
| `log.format` | `LOG_FORMAT` | Format (`console` / `json`) | `console` |
| `log.output` | `LOG_OUTPUT` | Output target (`stdout` / `file`) | `stdout` |
| `log.file_path` | - | File path when output is `file` | `./logs/app.log` |
| `log.max_size` | - | Max log file size (MB) before rotation | `100` |
| `log.max_age` | - | Max retention days for rotated files | `30` |
| `log.max_backups` | - | Max rotated backups to retain | `10` |
| `log.compress` | - | Compress rotated files with gzip | `true` |

Runtime logs emitted by Server, `openflare-edge`, workers, and schedulers stay in English regardless of UI locale, while console errors and task details localize dynamically (see [Runtime Language & Internationalization](../design/log-language.md)).

### 6. Async Task Worker Config (`worker:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `worker.concurrency` | `WORKER_CONCURRENCY` | Max concurrent worker goroutines | `20` |
| `worker.strict_priority` | `WORKER_STRICT_PRIORITY` | Enforce strict priority queue scheduling | `false` |
| `worker.queues` | - | Queue weights | `webhook: 10`, `whitelist_only: 5`, `default: 3` |

### 7. Tracing OpenTelemetry Config (`otel:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `otel.export_protocol` | `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` / `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP protocol (`grpc` / `http/protobuf`) | `grpc` |
| `otel.sampling_rate` | `OTEL_SAMPLING_RATE` | Trace sampling ratio (`0.0` disabled, `1.0` full) | `0.0` |
| `otel.tracer_name` | `OTEL_TRACER_NAME` | Tracer instrumentation name | `github.com/Rain-kl/OpenFlare` |

### 8. Embedded SmartDNS Config (`smartdns:`)
| YAML path | Override env var | Description | Default |
| --- | --- | --- | --- |
| `smartdns.enabled` | `SMARTDNS_ENABLED` | Enable embedded or supervised local SmartDNS engine in Server | `false` |
| `smartdns.listen_addr` | `SMARTDNS_LISTEN_ADDR` | Local authoritative DNS UDP/TCP bind address | `:53` |
| `smartdns.binary_path` | `SMARTDNS_BINARY_PATH` | Optional path to `openflare-dns`; uses embedded Go engine when empty | `""` |
| `smartdns.embedded_name` | `SMARTDNS_EMBEDDED_NAME` | Display name for the embedded instance | `Embedded SmartDNS` |
| `smartdns.data_dir` | `SMARTDNS_DATA_DIR` | Data directory for embedded SmartDNS state | `data/smartdns` |

---

## System Runtime Config (`SystemConfig`)

Stored in `w_system_configs` and dynamically invalidated across replicas and nodes via Redis.

### 1. Basic & Business Runtime Config
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `site_name` | `string` | Platform display name | `OpenFlare` |
| `server_address` | `string` | Public console URL for OAuth redirects and node install scripts | empty |
| `automation_public_url` | `string` | Public HTTPS URL for CLI & MCP (empty disables MCP/automation) | `""` |
| `password_login_enabled` | `bool` | Allow username/password login | `true` |
| `registration_enabled` | `bool` | Allow self-service user registration | `false` |
| `password_register_enabled` | `bool` | Allow direct frontend email/password sign-up | `false` |
| `oidc_login_enabled` | `bool` | Enable OIDC (SSO) sign-in | `true` |
| `max_api_keys_per_user` | `int` | Max personal access tokens per user | `5` |
| `login_session_ttl_hours` | `int` | Browser cookie session lifetime (hours; `0` = browser session) | `0` |
| `upload_allowed_extensions` | `string` | Comma-separated allowed upload extensions (empty = unlimited) | `jpg,png,webp` |
| `file_access_whitelist` | `json` | Upload categories accessible without authentication | `["avatar"]` |
| `disk_cache_max_size_mb` | `int` | Local disk cache threshold (MB) | `100` |
| `disk_cache_ttl_minutes` | `int` | Default cache entry TTL (minutes) | `60` |
| `disk_cache_lru_enabled` | `bool` | Evict oldest entries via LRU when threshold exceeded | `true` |
| `update_upstream_repository` | `string` | GitHub repository checked for updates | `Rain-kl/OpenFlare` |
| `storage_config` | `json` | Local disk or AWS S3-compatible upload storage configuration | local storage |
| `tunnel_update_repo` | `string` | Release repository checked by `openflare-relay` / `openflared` | `"Rain-kl/OpenFlare"` |
| `tunnel_heartbeat_interval` | `int` | Tunnel connector and relay heartbeat interval (ms) | `5000` |
| `tunnel_lease_ttl_seconds` | `int` | Relay scheduling lease TTL (seconds) | `30` |
| `tunnel_default_ha_relays` | `int` | Default HA relay count for new relay pools | `2` |
| `relay_discovery_token` | `string` | Auto-discovery bootstrap token for `openflare-relay` | `""` |
| `search_engine_indexing_enabled` | `bool` | Allow search engine indexing | `false` |
| `menu_display_config` | `string` | Menu visibility override map (`{url: enabled}`) | `{}` |
| `pages_max_package_size_mb` | `int` | Pages archive upload limit (`1–2048` MiB) | `100` |
| `pages_max_history_count` | `int` | Max retained Pages deployments per project (`0` = unlimited) | `20` |

### 2. Human Verification (PoW Captcha)
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `cap_login_enabled` | `bool` | Require PoW captcha on console login | `false` |
| `cap_auto_solve` | `bool` | Solve login captcha automatically in background | `true` |
| `cap_challenge_count` | `int` | Number of challenges per verification (`1–5`) | `1` |
| `cap_challenge_difficulty`| `int` | PoW hash difficulty (`3–5`) | `4` |
| `cap_challenge_size` | `int` | Challenge salt byte length | `32` |
| `cap_challenge_ttl_seconds`| `int` | Challenge lifetime (seconds) | `600` |
| `cap_token_ttl_seconds` | `int` | Verified login token TTL (seconds) | `1200` |

### 3. SMTP Mail Config
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `smtp_host` | `string` | Outbound SMTP host | empty |
| `smtp_port` | `int` | Outbound SMTP port (`465` SSL / `587` STARTTLS) | `465` |
| `smtp_username` | `string` | SMTP account username | empty |
| `smtp_password` | `string` | SMTP secret (masked after save) | empty |
| `email_login_verification_enabled` | `bool` | Require 6-digit email verification code on login | `false` |
| `email_register_verification_enabled` | `bool` | Require email verification code on self-registration | `false` |

### 4. Edge Node, SmartDNS & Network Intelligence Config
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `agent_discovery_token` | `string` | Global discovery token for initial `openflare-edge` enrollment | auto-generated |
| `agent_heartbeat_interval`| `int` | Heartbeat interval pushed to `openflare-edge` nodes (ms) | `3000` (3s) |
| `agent_websocket_upgrade_enabled` | `bool` | Allow `openflare-edge` WebSocket control channel upgrade | `true` |
| `node_offline_threshold` | `int` | Timeout before marking nodes offline (ms) | `60000` (60s) |
| `agent_update_repo` | `string` | Release repository checked by `openflare-edge` auto-update | `Rain-kl/OpenFlare` |
| `geoip_provider` | `string` | Default WAF GeoIP lookup provider | `ipinfo` |
| `smartdns_enabled` | `bool` | Enable authoritative SmartDNS feature | `true` |
| `smartdns_delegation_domain` | `string` | Global delegation zone domain (e.g. `edge.example.com`) | `""` |
| `smartdns_ns_hosts` | `json` | Authoritative NS hostnames (JSON array) | `[]` |
| `smartdns_default_ttl` | `int` | Default SmartDNS answer TTL (seconds) | `60` |
| `smartdns_discovery_token` | `string` | Standalone `openflare-dns` auto-discovery token | `""` |
| `smartdns_snapshot_interval_seconds` | `int` | Binary snapshot refresh interval (seconds) | `10` |
| `smartdns_update_repo` | `string` | Release repository checked by `openflare-dns` | `Rain-kl/OpenFlare` |
| `netintel_data_dir` | `string` | Local storage directory for GeoIP/BGP data | `data/netintel` |
| `netintel_geoip_source` | `string` | Network intelligence feed source | `fyralabs` |
| `netintel_bgptools_user_agent` | `string` | User-Agent sent to bgp.tools | `OpenFlare-NetIntel/1.0 (+https://github.com/Rain-kl/OpenFlare)` |
| `netintel_caida_enabled` | `bool` | Enable CAIDA AS relationship enrichment | `false` |
| `netintel_auto_probe_enabled` | `bool` | Enable periodic inter-node quality probing | `true` |

### 5. Uptime Kuma Monitoring Sync
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `uptime_kuma_enabled` | `bool` | Enable background diff-sync to Uptime Kuma | `false` |
| `uptime_kuma_url` | `string` | Uptime Kuma base URL | empty |
| `uptime_kuma_username` | `string` | Uptime Kuma admin username | empty |
| `uptime_kuma_password` | `string` | Uptime Kuma admin password (masked after save) | empty |
| `uptime_kuma_monitor_scope`| `string` | Monitor scope (`all` or `selected`) | `all` |
| `uptime_kuma_selected_sites`| `string` | Comma-separated site names when `selected` | empty |
| `uptime_kuma_sync_interval`| `int` | Sync cadence (minutes) | `5` |
| `uptime_kuma_interval` | `int` | Probe check interval (seconds) | `60` |
| `uptime_kuma_retry` | `int` | Retries before marking down | `0` |
| `uptime_kuma_retry_interval`| `int` | Delay between retries (seconds) | `60` |
| `uptime_kuma_timeout` | `int` | Probe request timeout (seconds) | `48` |

### 6. Edge Security, Rate Limits & Built-in Pages
Edge proxy parameters are distributed from console **Performance Policies** (see [Performance Policies](../guide/performance-policies.md)). The following defaults and edge response templates are included in distributed snapshots:

| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `edge_trusted_proxy_cidrs` | `string` | JSON array of CIDRs trusted to supply `CF-Connecting-IP` (`[]` disables trust) | 22 Cloudflare official ranges |
| `edge_default_limit_conn_per_server` | `int` | Default max concurrent connections per site (`0` disabled) | `0` |
| `edge_default_limit_conn_per_ip` | `int` | Default max concurrent connections per IP (`0` disabled) | `0` |
| `edge_default_limit_rate` | `string` | Default bandwidth cap per request (e.g. `512k`; empty disables) | empty |
| `edge_default_limit_req_per_ip` | `string` | Default per-IP request rate limit (e.g. `10r/s`; empty disables) | empty |
| `pow_page` | `json` | PoW challenge page preset/widget & custom HTML configuration | `{"template":"shield","widget":"progress"}` |
| `site_auth_html` | `string` | Custom HTML template for site password/auth prompt | `""` |
| `sw_offline_enabled` | `bool` | Enable Service Worker offline fallback page | `false` |
| `sw_offline_html` | `string` | Custom offline contact HTML (empty uses built-in template) | `""` |
| `sw_offline_domains` | `json` | JSON array of domains served by offline fallback | `[]` |

### 7. Origin Error Page
Applies to reverse-proxy routes only (see [Origin Error Page Design](../design/origin-error-page.md)).

| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | `bool` | Replace matched error bodies while preserving HTTP status code | `true` |
| `origin_error_page_get_only` | `bool` | Restrict custom error pages to `GET` requests | `false` |
| `origin_error_page_status_codes` | `json` | JSON list of single codes or `lo-hi` ranges within `400-599` | `["500-599"]` |
| `origin_error_page_html` | `string` | Custom HTML (`{{status}}`, `{{host}}`; max `256 KiB`) | empty |

### 8. Log Database
| Key | Type | Description | Default |
| --- | --- | --- | --- |
| `log_database` | `string` | Active log backend (`postgres` / `sqlite` / `clickhouse`; **internal protected key**) | follows primary / `clickhouse` |
| `log_db_migration` | `string` | Migration freeze flag (**internal protected key**; `migrating` rejects log writes with `503`) | empty |
| `log_retention_days_postgres` | `int` | PostgreSQL access log retention (days) | `30` |
| `log_retention_days_sqlite` | `int` | SQLite access log retention (days) | `30` |
| `log_retention_days_clickhouse` | `int` | ClickHouse access log retention (days) | `30` |
| `metric_retention_days` | `int` | Node telemetry retention across all backends (days) | `3` |

---

## Frontend Build Env Vars

| Env var | Purpose | Default |
| --- | --- | --- |
| `WAVELET_BACKEND_URL` | SSR and dev-proxy backend URL | `http://localhost:3000` |
| `NEXT_PUBLIC_WAVELET_BACKEND_URL` | Browser API backend URL (empty = same origin) | empty |
| `NEXT_PUBLIC_APP_VERSION` | Frontend version display | `dev` |

---

## OpenFlare Edge Flags & Env Vars

`openflare-edge run` supports these CLI flags and container entrypoint environment variables:

| CLI Flag | Container Env Var | Description & Default |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_EDGE_SERVER_URL` | Control-plane URL (required) |
| `--token-type` | `OPENFLARE_EDGE_TOKEN_TYPE` | `discovery` (default) or `node` |
| `--token-file` | `OPENFLARE_EDGE_TOKEN_FILE` | Path to auth token file (`OPENFLARE_EDGE_TOKEN` writes this file in Docker) |
| `--state-dir` | `OPENFLARE_EDGE_STATE_DIR` | Persistent state dir (Docker default `/var/lib/openflare-edge`) |
| `--http-listen` | `OPENFLARE_EDGE_HTTP_LISTEN` | HTTP bind socket (`127.0.0.1:8080` CLI, `0.0.0.0:8080` Docker, `0.0.0.0:80` installer) |
| `--tls-listen` | `OPENFLARE_EDGE_TLS_LISTEN` | HTTPS bind socket (`0.0.0.0:8443` Docker, `0.0.0.0:443` installer) |
| `--upgrade` | - | Enforce active snapshot recovery during upgrades |

---

## OpenFlare DNS Flags & Env Vars

`openflare-dns serve` supports matching `OPENFLARE_DNS_*` environment variables:

| CLI Flag | Env Var | Description & Default |
| --- | --- | --- |
| `--controller-url` | `OPENFLARE_DNS_CONTROLLER_URL` (fallback `OPENFLARE_SERVER_URL`) | Control-plane URL (required) |
| `--token` | `OPENFLARE_DNS_TOKEN` | Dedicated instance token |
| `--discovery-token` | `OPENFLARE_DNS_DISCOVERY_TOKEN` | Auto-discovery enrollment token |
| `--token-file` | `OPENFLARE_DNS_TOKEN_FILE` | Path to token file |
| `--token-type` | `OPENFLARE_DNS_TOKEN_TYPE` | Token type (`discovery` default) |
| `--instance-id` | `OPENFLARE_DNS_INSTANCE_ID` | Explicit instance ID override |
| `--name` | `OPENFLARE_DNS_NAME` | Instance display name (`Standalone SmartDNS`) |
| `--listen` | `OPENFLARE_DNS_LISTEN` | UDP/TCP DNS listen socket (`0.0.0.0:53`, container `0.0.0.0:5353`) |
| `--public-addr` | `OPENFLARE_DNS_PUBLIC_ADDR` | Public IP/address reported to controller |
| `--geoip-dir` | `OPENFLARE_DNS_GEOIP_DIR` | GeoIP database directory (`data/netintel`, container `/var/lib/openflare-dns/geoip`) |
| `--cache-dir` | `OPENFLARE_DNS_CACHE_DIR` | Snapshot and identity directory (`data/smartdns`, container `/var/lib/openflare-dns`) |
| `--workers` | `OPENFLARE_DNS_WORKERS` | Worker thread count (`0` = auto) |
| `--rrl-qps` / `--rrl-burst` | `OPENFLARE_DNS_RRL_QPS` / `OPENFLARE_DNS_RRL_BURST` | Response rate limits (`1000` QPS / `2000` burst) |
| `--snapshot-interval-secs` | `OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS` | Snapshot poll interval (`10`s) |

---

## Relay Flags & Env Vars

CLI flags and env vars supported by the Rust `openflare-relay` binary:

| CLI Flag | Env Var | Description & Default |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_SERVER_URL` | Control-plane URL (required) |
| `--token` / `--token-file` | `OPENFLARE_RELAY_TOKEN` / `OPENFLARE_RELAY_TOKEN_FILE` | Permanent relay token or file |
| `--bootstrap-token` | `OPENFLARE_RELAY_BOOTSTRAP_TOKEN` | Bootstrap/discovery registration token |
| `--state-dir` | `OPENFLARE_RELAY_STATE_DIR` | Persistent state dir (`/var/lib/openflare-relay`) |
| `--name` | `OPENFLARE_RELAY_NAME` | Relay display name |
| `--public-addr` (`--endpoint-host`) | `OPENFLARE_RELAY_PUBLIC_ADDR` | Public IP or hostname advertised to connectors |
| `--gateway-addr` | `OPENFLARE_RELAY_GATEWAY_ADDR` | Gateway address advertised to Edge nodes |
| `--zone-tag` | `OPENFLARE_RELAY_ZONE_TAG` | Failure-domain zone tag |
| `--colocated-edge-node-id` | `OPENFLARE_RELAY_COLOCATED_EDGE_NODE_ID` | Co-located OpenFlare Edge node ID |
| `--transport-port` | `OPENFLARE_RELAY_TRANSPORT_PORT` | Noise/TCP tunnel listener port (`7443`) |
| `--ws-port` | `OPENFLARE_RELAY_WS_PORT` | WebSocket fallback listener port (`7444`) |
| `--gateway-port` | `OPENFLARE_RELAY_GATEWAY_PORT` | L7 HTTP/HTTPS relay gateway port (`8443`) |

---

## OpenFlared Flags & Env Vars

CLI flags and env vars supported by the Rust `openflared` binary:

| CLI Flag | Env Var | Description & Default |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_SERVER_URL` | Control-plane URL (required) |
| `--token` / `--token-file` | `OPENFLARE_TUNNEL_TOKEN` / `OPENFLARE_TUNNEL_TOKEN_FILE` | Tunnel token or token file |
| `--bootstrap-token` | `OPENFLARE_TUNNEL_BOOTSTRAP_TOKEN` | Tunnel bootstrap token |
| `--state-dir` | `OPENFLARE_TUNNEL_STATE_DIR` | Persistent state dir (`/var/lib/openflared`) |
| `--connector-id` | `OPENFLARE_CONNECTOR_ID` | Optional connector ID override |
