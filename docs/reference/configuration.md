# 配置项

OpenFlare Server、前端构建、`openflare-edge`、`openflare-dns`、`openflare-relay` 与 `openflared` 支持的配置来源、字段、环境变量与默认值说明。

---

## 配置来源

### 1. Server 配置来源
- **配置文件**：启动时默认读取同级目录下的 `config.yaml`（可通过 `CONFIG_PATH` 环境变量指定）。
- **环境变量**：配置文件字段支持通过大写蛇形（`UPPER_SNAKE_CASE`）环境变量覆盖（环境变量优先级高于 `config.yaml`）。
- **系统运行时配置**：保存在关系数据库 `w_system_configs` 表中，支持通过管理端界面或 API 热更新生效。

### 2. 数据面与边缘组件配置来源
- **命令行参数与环境变量**：`openflare-edge`、`openflare-dns`、`openflare-relay` 与 `openflared` 均为 Rust 原生二进制，通过 CLI 参数与环境变量（如 `--server-url`、`OPENFLARE_*`）配置启动参数；除凭证文件外不依赖本地 JSON/YAML 主配置文件。
- **持久化状态目录**：运行时由组件自动在 `--state-dir` / `--cache-dir` 下维护节点身份与快照状态文件。

---

## 配置文件与状态文件位置

| 组件 | 默认路径 / 目录 | 说明 |
| --- | --- | --- |
| Server 配置文件 | `./config.yaml` | 可通过 `CONFIG_PATH` 修改 |
| Server SQLite 库 | `openflare.db` | 可通过 `database.sqlite_path` / `SQLITE_PATH` 修改 |
| Edge 状态目录 | `/var/lib/openflare-edge` | 由 `--state-dir` / `OPENFLARE_EDGE_STATE_DIR` 指定，内含 `identity.json`、`snapshot.json`、`bootstrap-token`、`ready.json` |
| SmartDNS 状态目录 | `/var/lib/openflare-dns`（容器） / `data/smartdns` | 由 `--cache-dir` / `OPENFLARE_DNS_CACHE_DIR` 指定，内含 `identity.json`、`last-snapshot.bin`、`ready.json` |
| SmartDNS GeoIP 目录 | `/var/lib/openflare-dns/geoip`（容器） / `data/netintel` | 由 `--geoip-dir` / `OPENFLARE_DNS_GEOIP_DIR` 指定 |
| Relay 状态目录 | `/var/lib/openflare-relay` | 由 `--state-dir` / `OPENFLARE_RELAY_STATE_DIR` 指定，内含 `relay_identity.json` |
| OpenFlared 状态目录 | `/var/lib/openflared` | 由 `--state-dir` / `OPENFLARE_TUNNEL_STATE_DIR` 指定，内含 `connector-id`、`connector_identity.json`、`bootstrap-token` |

---

## Server 命令行参数

```bash
# 启动 Server 时指定配置文件
CONFIG_PATH=/path/to/custom-config.yaml ./bin/openflare-server all
```

支持的子命令：
- `all`：单进程融合运行 API + Worker + Scheduler（默认）。
- `api`：仅启动控制面与节点通信 HTTP/WS API。
- `worker`：仅启动 Asynq 异步任务消费 Worker。
- `scheduler`：仅启动 Cron 定时调度器。
- `reset-passwd`：重置指定用户密码（支持 `--user` 与 `--password`）。
- `rotate-credentials`：预检（默认）或在单事务内重加密（`--apply`）库中加密凭证，用于安全轮换 `app.session_secret`。

---

## Server 环境变量与配置文件对照

Server 核心静态配置定义于 `config.yaml`，并支持通过环境变量覆盖（环境变量优先级高于 YAML）。

### 1. 应用与安全基本配置 (`enable_webshell` / `app:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `enable_webshell` | `ENABLE_WEBSHELL` | 是否允许服务端向 Edge 节点下发远程 Shell 执行任务（仅启动读取） | `false` |
| `app.app_name` | `APP_NAME` | 应用程序标识名称 | `openflare` |
| `app.env` | `APP_ENV` | 运行环境（`development` / `testing` / `production`） | `production` |
| `app.addr` | `APP_ADDR` | 服务监听地址与端口 | `:3000` |
| `app.node_id` | `APP_NODE_ID` | Snowflake 算法节点 ID（0-1023），多实例部署须唯一 | `1` |
| `app.api_prefix` | `APP_API_PREFIX` | 管理端与 API 路由前缀 | `/api` |
| `app.graceful_shutdown_timeout` | `APP_GRACEFUL_SHUTDOWN_TIMEOUT` | 优雅停机等待超时（秒） | `30` |
| `app.session_cookie_name` | `APP_SESSION_COOKIE_NAME` | 会话 Cookie 名称 | `openflare_session_id` |
| `app.session_secret` | `APP_SESSION_SECRET` | 会话签名与敏感字段加密密钥，**生产须设为随机长串（MCP 要求 >= 32 字节）** | 无（随机） |
| `app.session_domain` | `APP_SESSION_DOMAIN` | 共享 Session 的 Cookie 作用域域名 | 空 |
| `app.session_age` | `APP_SESSION_AGE` | 浏览器 Session 存活时间（秒） | `86400` (24h) |
| `app.session_http_only` | `APP_SESSION_HTTP_ONLY` | 是否启用 Cookie `HttpOnly` | `false` |
| `app.session_secure` | `APP_SESSION_SECURE` | 是否启用 Cookie `Secure`（HTTPS 下启用） | `false` |

### 2. 关系数据库配置 (`database:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `database.enabled` | `DB_ENABLED` | 是否启用 PostgreSQL；设为 `false` 回退 SQLite | `true` |
| `database.sqlite_path` | `SQLITE_PATH` | SQLite 数据库文件路径 | `openflare.db` |
| `database.host` | `DB_HOST` | PostgreSQL 地址 | `127.0.0.1` |
| `database.port` | `DB_PORT` | PostgreSQL 端口 | `5432` |
| `database.username` | `DB_USERNAME` | PostgreSQL 用户名 | `openflare` |
| `database.password` | `DB_PASSWORD` | PostgreSQL 密码 | `replace-with-strong-password` |
| `database.database` | `DB_NAME` | PostgreSQL 库名 | `openflare` |
| `database.max_idle_conn` | `DB_MAX_IDLE_CONN` | 连接池最大空闲连接数 | `16` |
| `database.max_open_conn` | `DB_MAX_OPEN_CONN` | 连接池最大打开连接数 | `128` |
| `database.conn_max_lifetime` | - | 连接最大存活时间（秒） | `1800` |
| `database.conn_max_idle_time` | - | 空闲连接最大空闲时间（秒） | `600` |
| `database.log_level` | `DB_LOG_LEVEL` | GORM SQL 日志等级（`info` / `warn` / `error` / `silent`） | `info` |
| `database.ssl_mode` | `DB_SSL_MODE` | PostgreSQL SSL 模式 | `disable` |
| `database.time_zone` | `DB_TIMEZONE` | 数据库会话时区 | `UTC` |
| `database.application_name` | - | 数据库连接应用标识 | `openflare-server` |
| `database.search_path` | - | PostgreSQL Schema 搜索路径 | `public` |
| `database.prefer_simple_protocol` | - | 是否优先使用简单查询协议 | `false` |
| `database.statement_cache_capacity`| - | 预编译 SQL 语句缓存容量 | `256` |
| `database.default_query_exec_mode` | - | 查询默认执行模式 | `cache_statement` |
| `database.slow_threshold` | - | 慢查询阈值（Duration） | `200ms` |

SQLite 文件库启动时自动开启 WAL 模式（内存库保留 `memory` 模式），请确保目录可写以创建 `-wal`/`-shm`；在线备份请使用管理端导出而非直接拷贝运行中 `.db` 文件。

### 3. Redis 配置 (`redis:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `redis.enabled` | `REDIS_ENABLED` | 是否启用 Redis（异步队列与同步必须开启） | `true` |
| `redis.addrs` | `REDIS_ADDR` | 单机或集群地址数组（环境变量仅设单地址） | `["127.0.0.1:6379"]` |
| `redis.username` | `REDIS_USERNAME` | Redis 用户名 | 空 |
| `redis.password` | `REDIS_PASSWORD` | Redis 访问密码 | 空 |
| `redis.db` | `REDIS_DB` | Redis 逻辑库编号 | `0` |
| `redis.cluster_mode` | - | 是否启用 Redis 集群模式 | `false` |
| `redis.master_name` | - | Sentinel 主节点名（非空启用哨兵） | 空 |
| `redis.key_prefix` | `REDIS_KEY_PREFIX` | Redis 键前缀 | `openflare:` |
| `redis.pool_size` | `REDIS_POOL_SIZE` | 连接池大小 | `100` |
| `redis.min_idle_conn` | - | 最小空闲连接数 | `10` |
| `redis.dial_timeout` | - | 建连超时（秒） | `5` |
| `redis.read_timeout` | - | 读超时（秒） | `3` |
| `redis.write_timeout` | - | 写超时（秒） | `3` |
| `redis.max_retries` | - | 失败重试次数 | `3` |
| `redis.pool_timeout` | - | 获取连接超时（秒） | `4` |
| `redis.conn_max_idle_time` | - | 空闲连接最大存活时间（秒） | `300` |
| `redis.maint_notifications` | `REDIS_MAINT_NOTIFICATIONS` | 是否开启 Redis maintenance notifications 自动协商；兼容不明时保持关闭 | `false` |

### 4. ClickHouse 配置 (`clickhouse:`)

> **说明**：以下为 OpenFlare 客户端连接参数；ClickHouse 服务端轻量调优可通过 `./config/clickhouse/performance.xml` 挂载至 `/etc/clickhouse-server/config.d/performance.xml`，详见 [启动 Server](../deployment/server.md)。

| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `clickhouse.enabled` | `CLICKHOUSE_ENABLED` | 是否启用 ClickHouse 存储指标与访问日志（未设或 `false` 时使用主库；设置 `true` 或 `CLICKHOUSE_HOST` 时启用） | `false` |
| `clickhouse.hosts` | `CLICKHOUSE_HOST` | 集群地址数组（环境变量仅设单地址） | `["127.0.0.1:9000"]` |
| `clickhouse.username` | `CLICKHOUSE_USERNAME` | 用户名 | `default` |
| `clickhouse.password` | `CLICKHOUSE_PASSWORD` | 密码 | `replace-with-clickhouse-password` |
| `clickhouse.database` | `CLICKHOUSE_NAME` | 数据库名称 | `openflare` |
| `clickhouse.max_idle_conn` | - | 客户端空闲连接数 | `8` |
| `clickhouse.max_open_conn` | - | 客户端最大打开连接数 | `16` |
| `clickhouse.conn_max_lifetime` | - | 连接最大存活时间（秒） | `3600` |
| `clickhouse.dial_timeout` | - | 建连超时（秒） | `5` |
| `clickhouse.block_buffer_size` | - | 原生协议 block 缓冲行数 | `32` |

### 5. 系统日志配置 (`log:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `log.level` | `LOG_LEVEL` | 日志等级（`debug` / `info` / `warn` / `error` / `fatal`） | `info` |
| `log.format` | `LOG_FORMAT` | 打印格式（`console` / `json`） | `console` |
| `log.output` | `LOG_OUTPUT` | 输出目标（`stdout` / `file`） | `stdout` |
| `log.file_path` | - | `file` 模式持久化路径 | `./logs/app.log` |
| `log.max_size` | - | 单文件切割阈值（MB） | `100` |
| `log.max_age` | - | 历史日志保留天数 | `30` |
| `log.max_backups` | - | 历史日志最大保留文件数 | `10` |
| `log.compress` | - | 是否 gzip 压缩切割日志 | `true` |

Server、`openflare-edge`、调度器与存储任务运行时日志统一采用英文，不随控制台界面语言变化；界面展示的接口错误与任务明细会按语言动态本地化，详见 [运行时语言与国际化](../design/log-language.md)。

### 6. 异步任务 Worker 配置 (`worker:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `worker.concurrency` | `WORKER_CONCURRENCY` | Asynq Worker 最大并发协程数 | `20` |
| `worker.strict_priority` | `WORKER_STRICT_PRIORITY` | 是否严格按队列优先级调度 | `false` |
| `worker.queues` | - | 队列名称与优先级权重 | `webhook: 10`, `whitelist_only: 5`, `default: 3` |

### 7. 链路追踪 OpenTelemetry 配置 (`otel:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `otel.export_protocol` | `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` / `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP 导出协议（`grpc` 或 `http/protobuf`） | `grpc` |
| `otel.sampling_rate` | `OTEL_SAMPLING_RATE` | 链路采样率（`0.0` 关闭，`1.0` 全量） | `0.0` |
| `otel.tracer_name` | `OTEL_TRACER_NAME` | OTel Tracer 标识名称 | `github.com/Rain-kl/OpenFlare` |

### 8. 内嵌 SmartDNS 配置 (`smartdns:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `smartdns.enabled` | `SMARTDNS_ENABLED` | 是否由 Server 内嵌或托管本地 SmartDNS 服务 | `false` |
| `smartdns.listen_addr` | `SMARTDNS_LISTEN_ADDR` | 本地权威 DNS 监听地址（UDP/TCP） | `:53` |
| `smartdns.binary_path` | `SMARTDNS_BINARY_PATH` | 可选 `openflare-dns` 二进制路径；留空使用内嵌 Go 引擎 | `""` |
| `smartdns.embedded_name` | `SMARTDNS_EMBEDDED_NAME` | 内嵌实例展示名称 | `Embedded SmartDNS` |
| `smartdns.data_dir` | `SMARTDNS_DATA_DIR` | 内嵌 SmartDNS 运行时数据目录 | `data/smartdns` |

---

## 运行时系统配置 (SystemConfig)

存储于主库 `w_system_configs` 表中，修改后通过 Redis 通知各节点与工作进程动态热更新。

### 1. 基础与业务运行时配置
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `site_name` | `string` | 控制台展示名称 | `OpenFlare` |
| `server_address` | `string` | 控制台公网访问地址（用于 OAuth 回调与接入命令生成） | 空 |
| `automation_public_url` | `string` | CLI / MCP 对外公开访问地址（留空则禁用 MCP 与自动化） | `""` |
| `password_login_enabled` | `bool` | 是否允许账号密码登录 | `true` |
| `registration_enabled` | `bool` | 是否允许自助注册 | `false` |
| `password_register_enabled` | `bool` | 是否允许前端邮箱/密码直接注册 | `false` |
| `oidc_login_enabled` | `bool` | 是否启用 OIDC (SSO) 登录 | `true` |
| `max_api_keys_per_user` | `int` | 单用户最大个人 API 访问令牌数 | `5` |
| `login_session_ttl_hours` | `int` | 会话有效期（小时，`0` 表示关闭浏览器失效） | `0` |
| `upload_allowed_extensions` | `string` | 允许上传的文件扩展名（空为不限） | `jpg,png,webp` |
| `file_access_whitelist` | `json` | 免登录可直接访问的文件业务类型列表 | `["avatar"]` |
| `disk_cache_max_size_mb` | `int` | 平台本地磁盘缓存上限（MB） | `100` |
| `disk_cache_ttl_minutes` | `int` | 磁盘缓存默认 TTL（分钟） | `60` |
| `disk_cache_lru_enabled` | `bool` | 磁盘缓存超限时是否按 LRU 淘汰 | `true` |
| `update_upstream_repository` | `string` | 版本检查 GitHub 仓库 | `Rain-kl/OpenFlare` |
| `storage_config` | `json` | 对象存储配置（支持本地与 S3 兼容存储） | 本地存储 |
| `tunnel_update_repo` | `string` | `openflare-relay` / `openflared` 更新查询仓库 | `"Rain-kl/OpenFlare"` |
| `tunnel_heartbeat_interval` | `int` | 隧道与中继心跳汇报间隔（毫秒） | `5000` |
| `tunnel_lease_ttl_seconds` | `int` | 中继调度租约 TTL（秒） | `30` |
| `tunnel_default_ha_relays` | `int` | 新建中继池默认高可用中继数 | `2` |
| `relay_discovery_token` | `string` | `openflare-relay` 自动发现引导令牌 | `""` |
| `search_engine_indexing_enabled` | `bool` | 是否允许搜索引擎收录 | `false` |
| `menu_display_config` | `string` | 侧边菜单可见性覆盖（JSON `{url: enabled}`） | `{}` |
| `pages_max_package_size_mb` | `int` | Pages 部署包大小上限（MiB，1–2048） | `100` |
| `pages_max_history_count` | `int` | Pages 单项目保留部署历史上限（0 为不限） | `20` |

### 2. 人机安全校验 (PoW Captcha)
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `cap_login_enabled` | `bool` | 登录界面是否开启算力人机验证 | `false` |
| `cap_auto_solve` | `bool` | 打开页面后是否自动在后台计算 | `true` |
| `cap_challenge_count` | `int` | 验证难题数量（推荐 1–5） | `1` |
| `cap_challenge_difficulty`| `int` | 前缀难度（推荐 3–5） | `4` |
| `cap_challenge_size` | `int` | 挑战盐值字节长度 | `32` |
| `cap_challenge_ttl_seconds`| `int` | 挑战有效期（秒） | `600` |
| `cap_token_ttl_seconds` | `int` | 通过验证后登录凭证有效期（秒） | `1200` |

### 3. SMTP 邮件推送配置
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `smtp_host` | `string` | SMTP 服务器地址 | 空 |
| `smtp_port` | `int` | SMTP 端口（常见 `465` SSL / `587` STARTTLS） | `465` |
| `smtp_username` | `string` | SMTP 发信账号 | 空 |
| `smtp_password` | `string` | SMTP 密码或授权码（保存后不回显） | 空 |
| `email_login_verification_enabled` | `bool` | 是否启用登录邮箱验证码二次校验 | `false` |
| `email_register_verification_enabled` | `bool` | 自助注册是否要求邮箱验证码 | `false` |

### 4. 边缘节点、SmartDNS 与网络情报配置
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `agent_discovery_token` | `string` | `openflare-edge` 首次接入全局发现令牌 | 自动生成 |
| `agent_heartbeat_interval`| `int` | 下发给 `openflare-edge` 的心跳上报间隔（毫秒） | `3000` (3s) |
| `agent_websocket_upgrade_enabled` | `bool` | 是否允许 `openflare-edge` 握手后升级为 WebSocket 长连接 | `true` |
| `node_offline_threshold` | `int` | 判定节点失联并标记离线的无响应阈值（毫秒） | `60000` (60s) |
| `agent_update_repo` | `string` | `openflare-edge` 自动更新查询的 Release 仓库 | `Rain-kl/OpenFlare` |
| `geoip_provider` | `string` | WAF 地域查询默认 GeoIP 提供方 | `ipinfo` |
| `smartdns_enabled` | `bool` | 是否启用 SmartDNS 权威智能解析功能 | `true` |
| `smartdns_delegation_domain` | `string` | 全局委派主域名（如 `edge.example.com`） | `""` |
| `smartdns_ns_hosts` | `json` | 权威 NS 主机名列表（JSON 数组） | `[]` |
| `smartdns_default_ttl` | `int` | SmartDNS 解析记录默认 TTL（秒） | `60` |
| `smartdns_discovery_token` | `string` | 独立 `openflare-dns` 自动发现注册令牌 | `""` |
| `smartdns_snapshot_interval_seconds` | `int` | 独立实例增量/不可变快照拉取周期（秒） | `10` |
| `smartdns_update_repo` | `string` | `openflare-dns` 版本检查仓库 | `Rain-kl/OpenFlare` |
| `netintel_data_dir` | `string` | IP 情报与 GeoIP/BGP 本地缓存目录 | `data/netintel` |
| `netintel_geoip_source` | `string` | 网络情报数据源名称 | `fyralabs` |
| `netintel_bgptools_user_agent` | `string` | 查询 bgp.tools 时的 User-Agent | `OpenFlare-NetIntel/1.0 (+https://github.com/Rain-kl/OpenFlare)` |
| `netintel_caida_enabled` | `bool` | 是否启用 CAIDA AS 关系增强 | `false` |
| `netintel_auto_probe_enabled` | `bool` | 是否启用节点间周期自动质量探测 | `true` |

### 5. Uptime Kuma 监控联动同步
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `uptime_kuma_enabled` | `bool` | 是否开启站点到 Uptime Kuma 的差量同步 | `false` |
| `uptime_kuma_url` | `string` | Uptime Kuma 访问 URL | 空 |
| `uptime_kuma_username` | `string` | 管理员账号 | 空 |
| `uptime_kuma_password` | `string` | 管理员密码（保存后脱敏） | 空 |
| `uptime_kuma_monitor_scope`| `string` | 同步范围（`all` / `selected`） | `all` |
| `uptime_kuma_selected_sites`| `string` | 指定站点名称逗号列表 | 空 |
| `uptime_kuma_sync_interval`| `int` | 后台对账同步周期（分钟） | `5` |
| `uptime_kuma_interval` | `int` | HTTP 探测间隔（秒） | `60` |
| `uptime_kuma_retry` | `int` | 失败重试次数 | `0` |
| `uptime_kuma_retry_interval`| `int` | 重试间隔（秒） | `60` |
| `uptime_kuma_timeout` | `int` | 单次探测超时（秒） | `48` |

### 6. 边缘安全、限流与内置响应页
OpenFlare 边缘参数与回源策略主要通过控制台「性能策略」统一下发（见 [OpenFlare Edge 性能策略](../guide/performance-policies.md)）。以下全局默认值与内置页面可在控制台维护并随快照同步到所有边缘节点：

| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `edge_trusted_proxy_cidrs` | `string` | 授权透传 `CF-Connecting-IP` 的可信代理 CIDR JSON 数组（`[]` 表示关闭） | Cloudflare 官方 22 个网段 |
| `edge_default_limit_conn_per_server` | `int` | 默认单站点并发连接上限（`0` 关闭） | `0` |
| `edge_default_limit_conn_per_ip` | `int` | 默认单 IP 并发连接上限（`0` 关闭） | `0` |
| `edge_default_limit_rate` | `string` | 默认单请求限速（如 `512k`，空关闭） | 空 |
| `edge_default_limit_req_per_ip` | `string` | 默认单 IP 请求速率限制（如 `10r/s`，空关闭） | 空 |
| `pow_page` | `json` | PoW 挑战页模板风格与自定义 HTML 配置 | `{"template":"shield","widget":"progress"}` |
| `site_auth_html` | `string` | 站点访问密码/身份认证页自定义 HTML | `""` |
| `sw_offline_enabled` | `bool` | 是否启用 Service Worker 离线兜底页 | `false` |
| `sw_offline_html` | `string` | 离线联系页自定义 HTML（空则用默认） | `""` |
| `sw_offline_domains` | `json` | 启用 Service Worker 离线兜底的域名 JSON 数组 | `[]` |

### 7. 源站错误页 (Origin Error Page)
仅作用于反向代理路由（不影响 Pages）。设计参见 [源站错误页设计](../design/origin-error-page.md)。

| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | `bool` | 是否启用全局源站错误页（保留原始 HTTP 状态码） | `true` |
| `origin_error_page_get_only` | `bool` | 是否仅替换 `GET` 请求响应体 | `false` |
| `origin_error_page_status_codes` | `json` | 触发状态码标签 JSON 数组（单码或 `lo-hi` 区间，范围 `400-599`） | `["500-599"]` |
| `origin_error_page_html` | `string` | 自定义 HTML（支持 `{{status}}`、`{{host}}`，上限 `256 KiB`） | 空 |

### 8. 日志存储（Log Database）
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `log_database` | `string` | 当前日志主库（`postgres` / `sqlite` / `clickhouse`，**内部受保护，仅迁移任务写入**） | 随主库 / `clickhouse` |
| `log_db_migration` | `string` | 日志库迁移冻结标志（**内部受保护**，置为 `migrating` 期间写日志返回 503） | 空 |
| `log_retention_days_postgres` | `int` | PostgreSQL 访问日志保留天数 | `30` |
| `log_retention_days_sqlite` | `int` | SQLite 访问日志保留天数 | `30` |
| `log_retention_days_clickhouse` | `int` | ClickHouse 访问日志保留天数 | `30` |
| `metric_retention_days` | `int` | 节点性能指标（CPU/内存/磁盘/网络）保留天数（三库共用） | `3` |

---

## 前端构建环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `WAVELET_BACKEND_URL` | 服务端渲染与开发代理访问的后端地址 | `http://localhost:3000` |
| `NEXT_PUBLIC_WAVELET_BACKEND_URL` | 浏览器请求 API 的后端地址；空为同源 | 空 |
| `NEXT_PUBLIC_APP_VERSION` | 前端展示版本号 | `dev` |

---

## OpenFlare Edge 启动参数与环境变量

`openflare-edge run` 通过以下 CLI 参数或容器环境变量启动：

| CLI 参数 | 容器环境变量 | 说明与默认值 |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_EDGE_SERVER_URL` | 控制面访问地址（必填） |
| `--token-type` | `OPENFLARE_EDGE_TOKEN_TYPE` | 令牌类型：`discovery`（默认）或 `node` |
| `--token-file` | `OPENFLARE_EDGE_TOKEN_FILE` | 令牌文件路径（容器中可通过 `OPENFLARE_EDGE_TOKEN` 自动落盘写入） |
| `--state-dir` | `OPENFLARE_EDGE_STATE_DIR` | 状态目录（容器默认 `/var/lib/openflare-edge`） |
| `--http-listen` | `OPENFLARE_EDGE_HTTP_LISTEN` | HTTP 监听地址（二进制默认 `127.0.0.1:8080`，容器 `0.0.0.0:8080`，安装脚本 `0.0.0.0:80`） |
| `--tls-listen` | `OPENFLARE_EDGE_TLS_LISTEN` | HTTPS 监听地址（容器默认 `0.0.0.0:8443`，安装脚本 `0.0.0.0:443`） |
| `--upgrade` | - | 存在本地有效快照时强制以平滑升级方式恢复配置 |

---

## OpenFlare DNS 参数与环境变量

`openflare-dns serve` 支持 CLI 参数及同名 `OPENFLARE_DNS_*` 环境变量：

| CLI 参数 | 环境变量 | 说明与默认值 |
| --- | --- | --- |
| `--controller-url` | `OPENFLARE_DNS_CONTROLLER_URL`（回退 `OPENFLARE_SERVER_URL`） | 控制面地址（必填） |
| `--token` | `OPENFLARE_DNS_TOKEN` | 节点或引导令牌 |
| `--discovery-token` | `OPENFLARE_DNS_DISCOVERY_TOKEN` | 自动发现注册令牌 |
| `--token-file` | `OPENFLARE_DNS_TOKEN_FILE` | 令牌文件路径 |
| `--token-type` | `OPENFLARE_DNS_TOKEN_TYPE` | 令牌类型（默认 `discovery`） |
| `--instance-id` | `OPENFLARE_DNS_INSTANCE_ID` | 指定实例 ID |
| `--name` | `OPENFLARE_DNS_NAME` | 实例展示名称（默认 `Standalone SmartDNS`） |
| `--listen` | `OPENFLARE_DNS_LISTEN` | DNS UDP/TCP 监听地址（默认 `0.0.0.0:53`，非特权容器默认 `0.0.0.0:5353`） |
| `--public-addr` | `OPENFLARE_DNS_PUBLIC_ADDR` | 节点公网 IP/地址上报覆盖 |
| `--geoip-dir` | `OPENFLARE_DNS_GEOIP_DIR` | GeoIP 数据目录（默认 `data/netintel`，容器 `/var/lib/openflare-dns/geoip`） |
| `--cache-dir` | `OPENFLARE_DNS_CACHE_DIR` | 快照与身份状态目录（默认 `data/smartdns`，容器 `/var/lib/openflare-dns`） |
| `--workers` | `OPENFLARE_DNS_WORKERS` | 工作线程数（默认 `0` 自动） |
| `--rrl-qps` / `--rrl-burst` | `OPENFLARE_DNS_RRL_QPS` / `OPENFLARE_DNS_RRL_BURST` | 响应速率限制（默认 `1000` QPS / `2000` burst） |
| `--snapshot-interval-secs` | `OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS` | 快照轮询间隔（默认 `10` 秒） |

---

## Relay 参数与环境变量

Rust `openflare-relay` 二进制支持的启动参数与环境变量：

| CLI 参数 | 环境变量 | 说明与默认值 |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_SERVER_URL` | 控制面地址（必填） |
| `--token` / `--token-file` | `OPENFLARE_RELAY_TOKEN` / `OPENFLARE_RELAY_TOKEN_FILE` | 中继永久凭证或凭证文件 |
| `--bootstrap-token` | `OPENFLARE_RELAY_BOOTSTRAP_TOKEN` | 注册引导/发现令牌 |
| `--state-dir` | `OPENFLARE_RELAY_STATE_DIR` | 状态目录（默认 `/var/lib/openflare-relay`） |
| `--name` | `OPENFLARE_RELAY_NAME` | 中继节点展示名 |
| `--public-addr` (`--endpoint-host`) | `OPENFLARE_RELAY_PUBLIC_ADDR` | 供连接器接入的公网 IP 或域名 |
| `--gateway-addr` | `OPENFLARE_RELAY_GATEWAY_ADDR` | 供 Edge 节点回源连接的网关地址 |
| `--zone-tag` | `OPENFLARE_RELAY_ZONE_TAG` | 故障域可用区标签 |
| `--colocated-edge-node-id` | `OPENFLARE_RELAY_COLOCATED_EDGE_NODE_ID` | 同机共置的 Edge 节点 ID |
| `--transport-port` | `OPENFLARE_RELAY_TRANSPORT_PORT` | Noise/TCP 控制与隧道端口（默认 `7443`） |
| `--ws-port` | `OPENFLARE_RELAY_WS_PORT` | WebSocket 回退隧道端口（默认 `7444`） |
| `--gateway-port` | `OPENFLARE_RELAY_GATEWAY_PORT` | L7 HTTP/HTTPS 网关监听端口（默认 `8443`） |

---

## OpenFlared 参数与环境变量

Rust `openflared` 二进制支持的启动参数与环境变量：

| CLI 参数 | 环境变量 | 说明与默认值 |
| --- | --- | --- |
| `--server-url` | `OPENFLARE_SERVER_URL` | 控制面地址（必填） |
| `--token` / `--token-file` | `OPENFLARE_TUNNEL_TOKEN` / `OPENFLARE_TUNNEL_TOKEN_FILE` | 隧道令牌或令牌文件 |
| `--bootstrap-token` | `OPENFLARE_TUNNEL_BOOTSTRAP_TOKEN` | 隧道首次引导令牌 |
| `--state-dir` | `OPENFLARE_TUNNEL_STATE_DIR` | 状态目录（默认 `/var/lib/openflared`） |
| `--connector-id` | `OPENFLARE_CONNECTOR_ID` | 可选连接器 ID 覆盖 |
