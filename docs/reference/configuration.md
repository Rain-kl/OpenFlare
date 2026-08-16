# 配置项

你会学到：OpenFlare Server、前端构建、Agent、Relay、OpenFlared 支持哪些配置来源、有哪些配置字段和环境变量，以及它们的默认值和行为。

本文档汇总了 OpenFlare 当前版本所支持的全部配置项。

---

## 配置来源

### 1. Server 配置来源
- **配置文件**：启动时默认读取同级目录下的 `config.yaml`（可通过 `CONFIG_PATH` 环境变量指定）。
- **环境变量**：所有配置文件中的字段均支持通过大写蛇形（`UPPER_SNAKE_CASE`）的环境变量进行覆盖（环境变量优先级高于 `config.yaml`）。
- **系统运行时配置**：保存在关系数据库的 `w_system_configs` 表中。此类参数可通过管理后台图形界面或系统 API 热更新并动态生效。

### 2. Agent / Relay / OpenFlared 配置来源
- **命令行参数**：通过 `-config` 指定配置文件（JSON 格式）。
- **配置文件**：例如 `agent.json`、`relay.json`、`flared.json`。
- **覆盖环境变量**：支持特定的环境变量来覆盖配置文件中的连接地址和 Token 凭证。

---

## 配置文件位置

| 组件 | 默认位置 | 说明 |
| --- | --- | --- |
| Server 配置文件 | `./config.yaml` | 可通过 `CONFIG_PATH` 环境变量修改 |
| Server SQLite 库 | `openflare.db` | 可通过 `database.sqlite_path` / `SQLITE_PATH` 修改 |
| Agent 配置文件 | `./agent.json` | 可通过 `-config` 指定 |
| 一键安装 Agent 配置 | `/opt/openflare-agent/agent.json` | 安装脚本默认生成路径 |
| Agent 数据目录 | 配置文件同级 `data` | 可在配置文件中通过 `data_dir` 覆盖 |
| Relay 配置文件 | `./relay.json` | 可通过 `-config` 指定 |
| 一键安装 Relay 配置 | `/opt/openflare-relay/relay.json` | 安装脚本默认生成路径 |
| Client 配置文件 | `./flared.json` | 可通过 `-config` 指定 |
| 一键安装 Client 配置 | `/opt/openflared/flared.json` | 安装脚本默认生成路径 |

---

## Server 命令行参数

```bash
# 启动 Server 时指定配置文件
CONFIG_PATH=/path/to/custom-config.yaml ./openflare-server all
```

运行支持的子服务指令（融合/单进程模式）：
- `all`：在一进程内启动所有服务（API + Worker + Scheduler，默认）。
- `api`：仅启动管理端与节点通信的 API 服务。
- `worker`：仅启动后台任务的 Worker 服务。
- `scheduler`：仅启动定时任务的 Scheduler 服务。

---

## Server 环境变量与配置文件对照

Server 的所有核心基础配置定义在 `config.yaml` 中，且均支持环境变量覆盖（变量优先级高于 YAML 配置文件）。

### 1. 应用基本配置 (`app:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `app.app_name` | `APP_NAME` | 应用程序标识名称 | `openflare` |
| `app.env` | `APP_ENV` | 运行环境（`development` / `testing` / `production`） | `production` |
| `app.addr` | `APP_ADDR` | 服务监听地址与端口 | `:3000` |
| `app.node_id` | `APP_NODE_ID` | Snowflake 算法的节点 ID（0-1023），多实例部署时必须唯一 | `1` |
| `app.api_prefix` | `APP_API_PREFIX` | 管理端与 API 的路由前缀 | `/api` |
| `app.graceful_shutdown_timeout` | `APP_GRACEFUL_SHUTDOWN_TIMEOUT` | 优雅停机等待超时（秒） | `30` |
| `app.session_cookie_name` | `APP_SESSION_COOKIE_NAME` | 会话 Cookie 的名称 | `openflare_session_id` |
| `app.session_secret` | `APP_SESSION_SECRET` | Session 会话签名的密钥，**生产环境必须配置为随机长字符串** | 无（随机） |
| `app.session_domain` | `APP_SESSION_DOMAIN` | 共享 Session 的 Cookie 作用域域名 | 空 |
| `app.session_age` | `APP_SESSION_AGE` | 浏览器 Session 的存活时间（秒） | `86400` (24h) |
| `app.session_http_only` | `APP_SESSION_HTTP_ONLY` | 是否启用 Cookie 的 HttpOnly 属性 | `false` |
| `app.session_secure` | `APP_SESSION_SECURE` | 是否启用 Cookie 的 Secure 属性（HTTPS 下使用） | `false` |

### 2. 关系数据库配置 (`database:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `database.enabled` | `DB_ENABLED` | 是否启用 PostgreSQL 数据库。设为 `false` 则回退到 SQLite | `true` |
| `database.sqlite_path` | `SQLITE_PATH` | PostgreSQL 禁用时，SQLite 数据库的文件路径 | `openflare.db` |
| `database.host` | `DB_HOST` | PostgreSQL 数据库连接地址 | `127.0.0.1` |
| `database.port` | `DB_PORT` | PostgreSQL 数据库端口 | `5432` |
| `database.username` | `DB_USERNAME` | PostgreSQL 数据库用户名 | `openflare` |
| `database.password` | `DB_PASSWORD` | PostgreSQL 数据库密码 | `replace-with-strong-password` |
| `database.database` | `DB_NAME` | PostgreSQL 数据库名 | `openflare` |
| `database.ssl_mode` | `DB_SSL_MODE` | PostgreSQL 的 SSL 模式 | `disable` |
| `database.time_zone` | `DB_TIMEZONE` | 数据库会话时区 | `UTC` |
| `database.log_level` | `DB_LOG_LEVEL` | GORM SQL 打印日志等级（`info` / `warn` / `error` / `silent`） | `info` |
| `database.max_idle_conn` | `DB_MAX_IDLE_CONN` | 数据库连接池最大空闲连接数 | `16` |
| `database.max_open_conn` | `DB_MAX_OPEN_CONN` | 数据库连接池最大打开连接数 | `128` |

### 3. Redis 配置 (`redis:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `redis.enabled` | `REDIS_ENABLED` | 是否启用 Redis 服务。**系统异步队列和同步依赖它，必须开启** | `true` |
| `redis.addrs` | `REDIS_ADDR` | Redis 单机或集群连接地址数组（环境变量仅设置单地址） | `["127.0.0.1:6379"]` |
| `redis.username` | `REDIS_USERNAME` | Redis 账号名称（若有） | 空 |
| `redis.password` | `REDIS_PASSWORD` | Redis 访问密码 | 空 |
| `redis.db` | `REDIS_DB` | Redis 逻辑数据库编号 | `0` |
| `redis.key_prefix` | `REDIS_KEY_PREFIX` | 系统在 Redis 中使用的键前缀 | `openflare:` |
| `redis.pool_size` | `REDIS_POOL_SIZE` | Redis 连接池大小 | `100` |
| `redis.maint_notifications` | `REDIS_MAINT_NOTIFICATIONS` | 启动时是否启用 Redis maintenance notifications 自动协商；兼容性不明时保持关闭，修改后需重启 | `false` |

### 4. ClickHouse 配置 (`clickhouse:`)

> **说明**：下列为 OpenFlare **客户端**连接参数。ClickHouse **服务端**小规格调优：将 `performance.xml` curl 到 `./config/clickhouse/`，再以单文件方式挂载至容器的 `config.d/performance.xml`，详见 [启动 Server](../deployment/server.md)。

| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `clickhouse.enabled` | `CLICKHOUSE_ENABLED` | 是否启用 ClickHouse。**系统节点指标与访问日志在此进行海量写入**。默认关闭：缺失本配置项或为 `false` 时不启用，日志/指标由主库承担；显式 `true` 或设置 `CLICKHOUSE_HOST` 时启用 | `false` |
| `clickhouse.hosts` | `CLICKHOUSE_HOST` | ClickHouse 集群连接地址数组（环境变量仅设置单地址） | `["127.0.0.1:9000"]` |
| `clickhouse.username` | `CLICKHOUSE_USERNAME` | ClickHouse 账号用户名 | `default` |
| `clickhouse.password` | `CLICKHOUSE_PASSWORD` | ClickHouse 密码 | `replace-with-clickhouse-password` |
| `clickhouse.database` | `CLICKHOUSE_NAME` | ClickHouse 存储的数据库名称 | `openflare` |
| `clickhouse.max_idle_conn` | - | 客户端空闲连接数（小规格默认偏低） | `8` |
| `clickhouse.max_open_conn` | - | 客户端最大打开连接数 | `16` |
| `clickhouse.conn_max_lifetime` | - | 连接最大存活时间（秒） | `3600` |
| `clickhouse.dial_timeout` | - | 建连超时（秒） | `5` |
| `clickhouse.block_buffer_size` | - | 原生协议 block 缓冲行数 | `32` |

### 5. 系统日志配置 (`log:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `log.level` | `LOG_LEVEL` | 全局日志记录等级（`debug` / `info` / `warn` / `error` / `fatal`） | `info` |
| `log.format` | `LOG_FORMAT` | 日志打印格式（`console` 易读控制台 / `json` 结构化） | `console` |
| `log.output` | `LOG_OUTPUT` | 日志输出渠道（`stdout` 标准输出 / `file` 文本文件） | `stdout` |
| `log.file_path` | - | 当 output 为 file 时日志的持久化路径 | `./logs/app.log` |
| `log.max_size` | - | 单个日志文件的最大空间（MB），超出自动切割轮转 | `100` |
| `log.max_age` | - | 历史切割日志文件最大保留天数 | `30` |

### 6. 异步任务 Worker 队列配置 (`worker:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `worker.concurrency` | `WORKER_CONCURRENCY` | 后台 Worker 进程同时消费任务的最大并发数 | `20` |
| `worker.strict_priority`| `WORKER_STRICT_PRIORITY` | 是否严格按队列优先级分配消费线程（否则为加权轮询） | `false` |

### 7. 链路追踪 OpenTelemetry 配置 (`otel:`)
| 配置文件 YAML 路径 | 对应覆盖环境变量 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `otel.sampling_rate` | `OTEL_SAMPLING_RATE` | OTel 链路追踪全局采样率。默认 `0.0` 不采样，`1.0` 为全量追踪 | `0.0` |
| `otel.tracer_name` | `OTEL_TRACER_NAME` | 全局 OTel 埋点 Tracer 的实例化名称 | `github.com/Rain-kl/OpenFlare` |

---

## 运行时系统配置 (SystemConfig)

这些配置项存储于关系型数据库中的 `w_system_configs` 表中。所有的配置项在修改后会主动通知 Redis 缓存失效以实现动态热更新，管理员可通过后台页面直接管理。

### 1. 基础与业务运行时配置
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `site_name` | `string` | 管理端平台的展示名称 | `OpenFlare` |
| `server_address` | `string` | 管理端控制台的对外公网访问地址，用于组装 OAuth 回调与下载链接 | 空 |
| `password_login_enabled` | `bool` | 是否允许管理员通过常规用户名密码方式登录后台 | `true` |
| `registration_enabled` | `bool` | 是否允许自助注册新用户（默认禁止，需由 root 账户邀请或分发） | `false` |
| `password_register_enabled` | `bool` | 是否允许通过邮箱/密码方式在前端直接注册 | `false` |
| `oidc_login_enabled` | `bool` | 是否启用 OIDC (SSO) 第三方免密登录方案 | `true` |
| `max_api_keys_per_user` | `int` | 每个后台用户可生成的最大 API 密钥（API Token）数量 | `5` |
| `login_session_ttl_hours` | `int` | 用户会话在浏览器 Cookie 中的有效期（小时）。0 为随浏览器关闭清除 | `0` |
| `upload_allowed_extensions` | `string` | 允许上传的文件扩展名（逗号分隔，空则不限） | `jpg,png,webp` |
| `file_access_whitelist` | `json` | 允许免登录直接公开下载或访问的文件业务类型列表 (JSON 数组) | `["avatar"]` |
| `disk_cache_max_size_mb` | `int` | 平台本地磁盘缓存的最大存储阈值（MB） | `100` |
| `disk_cache_ttl_minutes` | `int` | 本地磁盘缓存对象的默认生存周期（分钟） | `60` |
| `disk_cache_lru_enabled` | `bool` | 当本地磁盘缓存空间不足时是否启用 LRU 算法剔除最旧缓存 | `true` |
| `update_upstream_repository` | `string` | 系统检测自更新的 GitHub 仓库地址 | `Rain-kl/OpenFlare` |
| `storage_config` | `json` | 对象存储的结构化配置 (JSON)，支持本地磁盘与 AWS S3 兼容存储配置 | 本地存储模式 |
| `relay_frps_web_ui_enabled` | `bool` | 是否在中继节点上开启内嵌的 frps 流量监视面板 Web UI | `false` |
| `relay_frps_web_ui_port` | `int` | 中继节点 frps 监视面板所监听绑定的宿主机端口 | `17500` |
| `search_engine_indexing_enabled` | `bool` | 是否允许搜索引擎爬取/检索该站点 | `false` |
| `menu_display_config` | `string` | 目录显示的结构化配置 (JSON 字符串，格式为 `{url: enabled}`) | `{}` |
| `pages_max_package_size_mb` | `int` | Pages 部署包上传大小上限（MiB，范围 1～2048） | `100` |
| `pages_max_history_count` | `int` | Pages 每个项目最大历史部署保留数（0 表示不限制） | `20` |

### 2. 人机安全校验 (PoW Captcha)
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `cap_login_enabled` | `bool` | 是否在登录界面强制要求进行本地 PoW 算力防爆破人机验证 | `false` |
| `cap_auto_solve` | `bool` | 打开页面后是否由浏览器自动开始后台背景计算算力（无需用户手动点击）| `true` |
| `cap_challenge_count` | `int` | 人机验证所需的计算难题数。数量越大，计算要求时间越长（推荐 1～5） | `1` |
| `cap_challenge_difficulty`| `int`| 每次计算所需的 PoW 哈希前缀匹配难度。推荐数值在 3-5 之间 | `4` |
| `cap_challenge_size` | `int` | 人机验证盐值长度 | `32` |
| `cap_challenge_ttl_seconds`| `int`| 难题下发后等待计算提交的最长有效时间（秒），超时自动作废 | `600` |
| `cap_token_ttl_seconds` | `int` | 完成计算并置换到登录凭证后的有效期（秒），限制需在规定时间内登录 | `1200` |

### 3. SMTP 邮件推送配置
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `smtp_host` | `string` | 发信 SMTP 邮件服务器的连接地址 | 空 |
| `smtp_port` | `int` | 发信 SMTP 服务的端口 (通常是 465 SSL 或 587 STARTTLS) | `465` |
| `smtp_username` | `string` | SMTP 账户发信邮箱名称 | 空 |
| `smtp_password` | `string` | SMTP 账户的授权密码或证书密钥（后台写入后加密隐藏，不可回显） | 空 |
| `email_login_verification_enabled` | `bool` | 是否在用户邮箱登录时发送一次性 6 位动态验证码进行二次认证 | `false` |
| `email_register_verification_enabled` | `bool` | 用户自助注册时是否必须强制验证邮箱真实性并收取注册验证码 | `false` |

### 4. 节点与 Agent 运维运行时参数
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `agent_discovery_token` | `string` | 新节点首次一键接入并自动注册的全局通用验证发现 Token | 无（首次访问时自动生成） |
| `agent_heartbeat_interval`| `int` | 控制并向所有接入 Agent 周期下发的标准心跳检测间隔（毫秒） | `3000` (3s) |
| `agent_websocket_upgrade_enabled` | `bool` | 是否授权 Agent 在 HTTP 心跳握手成功后升级建立持久 WebSocket 实时连接 | `true` |
| `node_offline_threshold` | `int` | 在管理后台中判定节点失去心跳并标注为离线状态的无响应阈值（毫秒） | `60000` (60s) |
| `agent_update_repo` | `string` | Agent 节点更新下载自身二进制的 Release 仓库源 | `Rain-kl/OpenFlare` |
| `geoip_provider` | `string` | GeoIP 提供商，支持 `maxmind` 等，用于 WAF 防护时地域分析 | `ipinfo` |

### 5. Uptime Kuma 监控联动同步
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `uptime_kuma_enabled` | `bool` | 是否在配置发布激活后自动同步生成 Uptime Kuma 对应的 HTTP 监控项 | `false` |
| `uptime_kuma_url` | `string` | Uptime Kuma 控制端实例的访问 URL (含端口与路径) | 空 |
| `uptime_kuma_username` | `string` | Uptime Kuma 后台用于同步接口调用认证的管理员账号名称 | 空 |
| `uptime_kuma_password` | `string` | Uptime Kuma 后台对应的登录密码（后台写入后加密隐藏，不可回显） | 空 |
| `uptime_kuma_monitor_scope`| `string` | 自动生成监控项的路由范围（支持 `all` 全选网站或 `selected` 选定部分） | `all` |
| `uptime_kuma_selected_sites`| `string` | 选定进行监控的代理网站 Site Name 名称列表（英文逗号分隔） | 空 |
| `uptime_kuma_sync_interval`| `int` | 向 Uptime Kuma 实例进行差异扫描并校准同步的频率间隔（分钟） | `5` |
| `uptime_kuma_interval` | `int` | 生成的 HTTP 监控对象发出 HTTP GET 探测的周期检测频率（秒） | `60` |
| `uptime_kuma_retry` | `int` | HTTP 监控对象在遭遇连接波动失败后的最大重试重连次数 | `0` |
| `uptime_kuma_retry_interval`| `int` | HTTP 监控对象失败重连重试的间隔停顿时间（秒） | `60` |
| `uptime_kuma_timeout` | `int` | 触发 HTTP GET 监控请求判定超时的断开限制时间（秒） | `48` |

### 6. OpenResty 核心主配置与渲染选项 (OpenResty Config)
| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `openresty_default_server_return_status` | `int` | 默认未命中匹配路由的 HTTP 请求返回的响应状态码 | `421` |
| `openresty_worker_processes` | `string` | nginx `worker_processes` 参数设置。支持固定整数值或自动分配 `auto` | `auto` |
| `openresty_worker_connections` | `int` | nginx `worker_connections` 单进程最大连接承载限制 | `4096` |
| `openresty_worker_rlimit_nofile` | `int` | nginx `worker_rlimit_nofile` 能够打开的最大物理文件描述符限制 | `65535` |
| `openresty_events_use` | `string` | 绑定的事件轮询引擎（例如 Linux 下首选 `epoll`） | `epoll` |
| `openresty_events_multi_accept_enabled` | `bool` | 允许 worker 进程单次批量接受所有挂起的网络握手请求 | `true` |
| `openresty_keepalive_timeout` | `int` | nginx 连接保持连接复用的 `keepalive_timeout` 限制时长（秒） | `20` |
| `openresty_keepalive_requests` | `int` | 单一 TCP 连接复用过程中被允许的最大累计请求处理次数 | `1000` |
| `openresty_client_header_timeout` | `int` | 接收客户端整个 Request Header 头信息的读取超时上限时长（秒） | `15` |
| `openresty_client_body_timeout` | `int` | 接收客户端 Request Body 载荷体的数据读取超时上限时长（秒） | `15` |
| `openresty_client_max_body_size` | `string` | 允许客户端请求上传的最大 Body 大小限制，通常需要单位如 `10m`/`50m` | `64m` |
| `openresty_large_client_header_buffers` | `string` | 复杂请求超大请求头的专属缓冲区数目与大小（如 `4 16k`） | `4 16k` |
| `openresty_send_timeout` | `int` | 向客户端推送 Response 数据回执单次传输最大的间隔超时时长（秒）| `30` |
| `openresty_resolvers` | `string` | 节点进行 DNS 域名动态解析所关联绑定的域名解析器地址与配置参数 | 空 |
| `openresty_proxy_connect_timeout` | `int` | 向后台代理源站发起 TCP 三次握手建连的最长超时上限时长（秒） | `3` |
| `openresty_proxy_send_timeout` | `int` | 向源站单次写入并发送请求流数据的最大写入操作间隔时长（秒） | `60` |
| `openresty_proxy_read_timeout` | `int` | 源站收到请求后返回数据，Agent 最大的等待数据返回间隔时长（秒） | `60` |
| `openresty_websocket_enabled` | `bool` | 是否在 HTTP 段中自动载入渲染支持 WebSocket 协议的全局变量及头信息 | `true` |
| `openresty_http3_enabled` | `bool` | 是否在生成 nginx 监听描述中渲染支持 HTTP/3 QUIC 双栈监听能力 | `true` |
| `openresty_proxy_request_buffering_enabled`| `bool` | 是否将客户端 Request Body 先在网关做完全部读取缓存再向源站递交 | `false` |
| `openresty_proxy_buffering_enabled` | `bool` | 是否允许网关暂存源站的大量 Response 数据待全部解析后再转发给用户 | `true` |
| `openresty_proxy_buffers` | `string` | nginx 反代响应缓冲区的分配数量与单缓存大大小（如 `16 16k`） | `16 16k` |
| `openresty_proxy_buffer_size` | `string` | 存放源站返回 Response Header 头部信息的专属缓冲区限制 | `8k` |
| `openresty_proxy_busy_buffers_size` | `string` | 响应数据流返回过大时限制网关处于 Busy 状态的缓冲上限 | `64k` |
| `openresty_gzip_enabled` | `bool` | 是否在网关对符合条件的内容启用 gzip 编码实时压缩返回 | `true` |
| `openresty_gzip_min_length` | `int` | 触发 gzip 实时压缩的文件大小门槛。低于此大小无需压缩浪费 CPU | `1024` (1KB) |
| `openresty_gzip_comp_level` | `int` | gzip 压缩强度等级。支持 1-9，数字越大压缩率越高，越消耗算力 | `5` |
| `openresty_cache_enabled` | `bool` | 是否在全局配置中初始化代理缓存区域（Proxy Cache Path） | `false` |
| `openresty_cache_path` | `string` | 节点上代理缓存存放的临时物理目录路径 | `__OPENFLARE_PROXY_CACHE_PATH__` |
| `openresty_cache_levels` | `string` | 代理缓存的存储目录树层级分配设置 | `1:2` |
| `openresty_cache_inactive` | `string` | 缓存文件多长时间无人访问后将自动从磁盘上失效抹除的时间时长 | `30m` (30分钟) |
| `openresty_cache_max_size` | `string` | 代理缓存区域在节点上占用的最大可用物理磁盘额度 | `1g` (1GB) |
| `openresty_cache_key_template` | `string` | 默认生成代理缓存键的识别模板 | `$scheme$host$request_uri` |
| `openresty_cache_lock_enabled` | `bool` | 遭遇高并发请求击穿同一失效资源时是否对向源站发起建连排队加锁 | `true` |
| `openresty_cache_lock_timeout` | `string` | 抢夺代理缓存锁排队建连时排队等待的最长等待耗时限制 | `5s` |
| `openresty_cache_use_stale` | `string` | 当源站遇到特定报错（如500/502/504等）时是否直接向用户投递过期缓存 | `error timeout updating http_500 http_502 http_503 http_504` |
| `openresty_default_limit_conn_per_server` | `int` | 站点未配置时的默认并发连接上限；`0` 表示默认关闭 | `0` |
| `openresty_default_limit_conn_per_ip` | `int` | 站点未配置时的默认单 IP 并发上限；`0` 表示默认关闭 | `0` |
| `openresty_default_limit_rate` | `string` | 站点未配置时的默认单请求带宽（如 `512k`）；空表示默认关闭 | 空 |
| `openresty_default_limit_req_per_ip` | `string` | 站点未配置时的默认单 IP 请求频率限制（如 `10r/s`、`100r/m`）；空表示默认关闭 | 空 |
| `openresty_main_config_template` | `string` | 允许用户完全重写整个 OpenResty nginx.conf 的底层结构大骨架模板 | 空 (内置缺省骨架) |

### 7. 源站错误页 (Origin Error Page)

全局源站错误页配置，写入配置版本快照后随发布/回滚下发到边缘 Agent。仅作用于**反代**路由；Pages 静态路由不受影响。管理端入口：「网站管理 → 错误页」。设计说明见 [源站错误页设计](../design/origin-error-page.md)。

| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | `bool` | 是否启用全局源站错误页。开启后，源站或网关返回的匹配状态码由自定义/默认 HTML 替换，**HTTP 状态码保持原值**；关闭后不生成相关指令，恢复透传。修改后需发布配置版本生效 | `true` |
| `origin_error_page_get_only` | `bool` | 是否仅对 **GET** 请求生效。开启后仅 GET 的匹配错误状态码返回自定义错误页；POST/PUT 等其它方法**透传源站响应**（原始状态码与响应体不变） | `false` |
| `origin_error_page_status_codes` | `json` | 触发错误页的状态码标签 JSON 数组。支持单码（如 `522`）与闭区间（如 `500-599`）；单码与区间两端均须在 **400–599**，且 `lo ≤ hi`。启用时展开结果不能为空 | `["500-599"]` |
| `origin_error_page_html` | `string` | 错误页自定义 HTML。空字符串表示使用内置 OpenFlare 默认模板（极简白底）；支持占位符 `{{status}}`（与 HTTP 状态码一致）、`{{host}}`（请求 Host）。最大 **256 KiB**（按字节）。勿嵌入不可信第三方脚本 | 空 |

---

### 8. 日志存储（Log Database）

日志存储解耦后的运行时配置：日志主库由「切换日志数据库」任务管理（内部/受保护 key，禁止管理员手动修改），访问日志保留天数按存储库分别在业务配置中设置；性能指标（CPU/内存/磁盘/网络）价值衰减快，按三库共用的独立短留存清理。

| 配置键 (Key) | 数据类型 | 作用说明 | 默认值 |
| --- | --- | --- | --- |
| `log_database` | `string` | 当前日志主库（`postgres` / `sqlite` / `clickhouse`）。**内部受保护 key**：仅「切换日志数据库」迁移任务写入，管理员不可手动创建/修改 | 随主库（PostgreSQL 启用时为 `postgres`，否则 `sqlite`；ClickHouse 启用时优先 `clickhouse`） |
| `log_db_migration` | `string` | 日志迁移冻结标记（`migrating` 或空）。**内部受保护 key**：仅迁移任务写入，置位期间日志写入返回 503「日志数据库迁移中，暂不可写」 | 空 |
| `log_retention_days_postgres` | `int` | PostgreSQL 日志库的访问日志过期清理保留天数（过期日志由系统垃圾清理每日任务删除） | `30` |
| `log_retention_days_sqlite` | `int` | SQLite 日志库的访问日志过期清理保留天数 | `30` |
| `log_retention_days_clickhouse` | `int` | ClickHouse 日志库的访问日志过期清理保留天数 | `30` |
| `metric_retention_days` | `int` | 性能指标（CPU/内存/磁盘/网络）保留天数，三库共用独立短留存（不随访问日志保留配置） | `3` |

---

## 前端构建环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `WAVELET_BACKEND_URL` | 服务端渲染与开发代理访问的后端地址 | `http://localhost:3000` |
| `NEXT_PUBLIC_WAVELET_BACKEND_URL` | 浏览器端请求 API 的后端地址；为空时使用同源 | 空 |
| `NEXT_PUBLIC_APP_VERSION` | 前端展示版本号 | `dev` |

---

## Agent 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `LOG_LEVEL` | Agent 日志等级 | `info` |
| `OPENFLARE_SERVER_URL` | 控制面地址，可覆盖 `agent.json` | 空 |
| `OPENFLARE_AGENT_TOKEN` | 节点专属认证 Token，可覆盖 `agent.json` | 空 |
| `OPENFLARE_DISCOVERY_TOKEN` | 首次自动注册 Token，可覆盖 `agent.json` | 空 |
| `OPENFLARE_NODE_NAME` | 节点名称，可覆盖 `agent.json` | 空 |
| `OPENFLARE_NODE_IP` | 节点 IP，可覆盖 `agent.json` | 空 |
| `OPENFLARE_DATA_DIR` | Agent 数据目录，可覆盖 `agent.json` | 空 |
| `OPENFLARE_OPENRESTY_PATH` | OpenResty 二进制路径，可覆盖 `agent.json` | 空 |
| `OPENFLARE_PAGES_DIR` | Pages 静态部署目录，可覆盖 `agent.json` | 空 |
| `OPENFLARE_HEARTBEAT_INTERVAL` | 心跳间隔，可覆盖 `agent.json` | 空 |
| `OPENFLARE_REQUEST_TIMEOUT` | 请求超时，可覆盖 `agent.json` | 空 |
| `OPENFLARE_OPENRESTY_OBSERVABILITY_PORT` | 本地观测端口，可覆盖 `agent.json` | 空 |
| `OPENFLARE_MMDB_PATH` | WAF GeoIP mmdb 路径，可覆盖 `agent.json` | 空 |
| `OPENFLARE_MMDB_UPDATE_INTERVAL` | WAF GeoIP mmdb 更新间隔，可覆盖 `agent.json` | 空 |
| `OPENFLARE_MMDB_DOWNLOAD_URL` | WAF GeoIP mmdb 下载地址，可覆盖 `agent.json` | 空 |
| `OPENFLARE_CITY_MMDB_PATH` | WAF 地区匹配 City MMDB 路径，可覆盖 `agent.json` | 空 |
| `OPENFLARE_CITY_MMDB_DOWNLOAD_URL` | WAF City MMDB 下载地址，可覆盖 `agent.json` | 空 |

---

## Agent 命令行参数与配置字段

### 命令行参数
- `-config`：指定 Agent 配置文件路径，默认值为 `./agent.json`。

### 配置文件字段 (agent.json)
| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `agent_token` | 节点专属认证 Token | 与 `discovery_token` 二选一 | 空 |
| `discovery_token` | 首次自动注册使用的全局 Token | 与 `agent_token` 二选一 | 空 |
| `node_name` | 节点名称 | 否 | 自动使用主机名 |
| `node_ip` | 节点 IP | 否 | 自动探测，优先使用公网出口 IP；失败时退回本机网卡探测 |
| `openresty_path` | OpenResty 二进制路径 | 否 | `openresty` |
| `openresty_observability_port` | 本地观测与 OpenResty 健康检查端口 | 否 | `18081` |
| `data_dir` | Agent 数据目录 | 否 | 配置文件同级目录下的 `data` |
| `main_config_path` | OpenResty 主配置写入路径 | 否 | `data_dir/etc/nginx/nginx.conf` |
| `route_config_path` | 路由配置写入路径 | 否 | `data_dir/etc/nginx/conf.d/openflare_routes.conf` |
| `access_log_path` | OpenResty 访问日志路径 | 否 | `data_dir/var/log/openflare/access.log` |
| `cert_dir` | 证书写入目录 | 否 | `data_dir/etc/nginx/certs` |
| `openresty_cert_dir` | OpenResty 配置中读取证书的目录 | 否 | 同 `cert_dir` |
| `lua_dir` | Lua 脚本与静态资源写入目录 | 否 | `data_dir/etc/nginx/lua` |
| `openresty_lua_dir` | OpenResty 配置中读取 Lua 的目录 | 否 | 同 `lua_dir` |
| `runtime_config_dir` | Agent 运行时配置写入目录，如 `pow_config.json` | 否 | `data_dir/etc/openflare` |
| `pages_dir` | Pages 静态部署包解压与当前部署目录 | 否 | `data_dir/var/lib/openflare/pages` |
| `mmdb_path` | WAF GeoIP mmdb 文件路径 | 否 | `data_dir/etc/openflare/GeoLite2-Country.mmdb` |
| `city_mmdb_path` | WAF 地区匹配 City MMDB 文件路径 | 否 | `data_dir/etc/openflare/GeoLite2-City.mmdb` |
| `mmdb_update_interval` | WAF GeoIP mmdb 更新间隔 | 否 | `86400000` 毫秒 (24h) |
| `mmdb_download_url` | WAF GeoIP mmdb 周期更新地址 | 否 | GeoLite2 Country 更新地址；磁盘文件缺失时首次下载（Docker 镜像已 COPY 默认路径文件） |
| `city_mmdb_download_url` | WAF City MMDB 周期更新地址 | 否 | GeoLite2 City 更新地址；磁盘文件缺失时首次下载（Docker 镜像已 COPY 默认路径文件） |
| `observability_buffer_path` | 观测补报缓冲文件路径 | 否 | `data_dir/var/lib/openflare/observability-buffer.json` |
| `observability_replay_minutes` | 自动补传最近观测窗口分钟数 | 否 | `60` |
| `state_path` | Agent 本地状态文件路径 | 否 | `data_dir/var/lib/openflare/agent-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `3000` 毫秒 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒 |

---

## Relay 环境变量与配置字段

### 环境变量
- `LOG_LEVEL`：Relay 日志等级，默认 `info`。
- 支持 `OPENFLARE_SERVER_URL`、`OPENFLARE_AGENT_TOKEN`、`OPENFLARE_DISCOVERY_TOKEN`、`OPENFLARE_NODE_NAME`、`OPENFLARE_NODE_IP`、`OPENFLARE_DATA_DIR`、`OPENFLARE_FRPS_PATH` 环境变量覆盖。

### 命令行参数
- `-config`：指定 Relay 配置文件路径，默认 `./relay.json`。

### 配置文件字段 (relay.json)
| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `agent_token` | 中继节点专属认证 Token | 与 `discovery_token` 二选一 | 空 |
| `discovery_token` | 首次自动注册使用的全局 Token | 与 `agent_token` 二选一 | 空 |
| `node_name` | 节点名称 | 否 | 自动使用主机名 |
| `node_ip` | 中继节点 IP，用于接收穿透流量 | 否 | 自动探测，优先使用公网出口 IP；失败时退回网卡探测 |
| `frps_path` | frps 二进制路径 | 否 | `frps`（在系统 PATH 中寻找） |
| `data_dir` | Relay 运行时数据目录 | 否 | 配置文件所在目录下的 `data` |
| `state_path` | Relay 本地状态文件存储路径 | 否 | `data_dir/relay-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒 |

---

## OpenFlared (Client) 环境变量与配置字段

### 环境变量
- `LOG_LEVEL`：Client 日志等级，默认 `info`。
- 支持 `OPENFLARE_SERVER_URL`、`OPENFLARE_TUNNEL_TOKEN`、`OPENFLARE_DATA_DIR`、`OPENFLARE_FRPC_PATH` 环境变量覆盖。

### 命令行参数
- `-config`：指定 Client 配置文件路径，默认 `./flared.json`。

### 配置文件字段 (flared.json)
| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `tunnel_token` | 隧道专属认证 Token | 是 | 无 |
| `frpc_path` | frpc 二进制路径 | 否 | `frpc`（在系统 PATH 中寻找） |
| `data_dir` | Client 运行时数据目录 | 否 | 配置文件所在目录下的 `data` |
| `state_path` | Client 本地状态文件存储路径 | 否 | `data_dir/flared-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒 |
| `sync_interval` | 配置拉取同步间隔 | 否 | `30000` 毫秒 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒 |
