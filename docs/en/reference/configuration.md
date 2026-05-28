# Configuration

## Server CLI Flags

| Flag | Purpose | Default |
| --- | --- | --- |
| `--port` | Server listen port | `3000` |
| `--log-dir` | Log directory | empty |
| `--version` | Print version and exit | `false` |
| `--help` | Print help and exit | `false` |

## Server Environment Variables

| Variable | Purpose | Default |
| --- | --- | --- |
| `PORT` | Server listen port | `3000` |
| `GIN_MODE` | Gin mode | release unless `debug` |
| `LOG_LEVEL` | Log level | `info` |
| `SESSION_SECRET` | Session signing secret | random on startup |
| `SQLITE_PATH` | SQLite database path | `openflare.db` |
| `DSN` | PostgreSQL DSN, preferred over SQLite | empty |
| `SQL_DSN` | Legacy PostgreSQL DSN, lower priority than `DSN` | empty |
| `REDIS_CONN_STRING` | Redis connection string | empty |
| `UPLOAD_PATH` | Upload directory | `upload` |
| `AGENT_TOKEN` | Legacy global Agent token | empty |

When `DSN` and `SQL_DSN` both exist, `DSN` wins. PostgreSQL is preferred when configured. If PostgreSQL is empty and a local SQLite file exists, Server migrates SQLite data at startup.

## Frontend Build Variables

| Variable | Purpose | Default |
| --- | --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | Frontend API base path | `/api` |
| `NEXT_PUBLIC_APP_VERSION` | Displayed frontend version | `dev` |
| `NEXT_DEV_BACKEND_URL` | Local dev backend proxy target | `http://127.0.0.1:3000` |

## Runtime Options

The settings page maintains these hot-updatable options:

| Option | Purpose | Default |
| --- | --- | --- |
| `AgentHeartbeatInterval` | Agent heartbeat interval in milliseconds | `10000` |
| `NodeOfflineThreshold` | Node offline threshold in milliseconds | `120000` |
| `AgentUpdateRepo` | Agent update repository | `Rain-kl/OpenFlare` |
| `GeoIPProvider` | Node/IP region provider | `ipinfo` |
| `DatabaseAutoCleanupEnabled` | Enable daily observability cleanup | `false` |
| `DatabaseAutoCleanupRetentionDays` | Retention days | `30` |

OpenResty performance and cache options are also stored in the Option table, including `OpenRestyWorkerProcesses`, `OpenRestyWorkerConnections`, `OpenRestyProxyConnectTimeout`, `OpenRestyProxyReadTimeout`, `OpenRestyCacheEnabled`, `OpenRestyCachePath`, and `OpenRestyCacheMaxSize`.

## Agent Configuration

Agent supports the `-config` CLI flag, an `agent.json` file, and the `LOG_LEVEL` environment variable.

| Field | Purpose | Required | Default / behavior |
| --- | --- | --- | --- |
| `server_url` | Control plane URL | yes | none |
| `agent_token` | Node-specific auth token | one of `agent_token` / `discovery_token` | empty |
| `discovery_token` | Global token for first registration | one of `agent_token` / `discovery_token` | empty |
| `node_name` | Node name | no | host name |
| `node_ip` | Node IP | no | auto-detected |
| `openresty_path` | OpenResty binary path | no | `openresty` |
| `openresty_container_name` | Deprecated Docker-control field, read for compatibility only | no | empty |
| `openresty_docker_image` | Deprecated Docker-control field, read for compatibility only | no | empty |
| `openresty_observability_port` | Local observability port | no | `18081` |
| `docker_binary` | Deprecated Docker-control field, read for compatibility only | no | empty |
| `data_dir` | Agent data directory | no | `data` under config directory |
| `access_log_path` | OpenResty access log path | no | `data_dir/var/log/openflare/access.log` |
| `runtime_config_dir` | Runtime config directory, including `pow_config.json` | no | `data_dir/etc/openflare` |
| `heartbeat_interval` | Heartbeat interval | no | `10000` ms |
| `request_timeout` | HTTP timeout | no | `10000` ms |

`heartbeat_interval` and `request_timeout` accept milliseconds or Go duration strings.
