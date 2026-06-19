# 配置项

你会学到：OpenFlare Server、前端构建和 Agent 支持哪些配置来源、配置项默认值是什么，以及常见部署组合应该如何配置。

本文档汇总 OpenFlare `1.0.0` 当前支持的 Server 与 Agent 配置项，只保留仍然有效的启动、部署与运行参数。

## 配置来源

Server 支持三类配置来源：

1. 命令行参数。
2. 环境变量。
3. 数据库 `Option` 表中的运行时配置。

Agent 支持：

1. `-config` 命令行参数。
2. `agent.json` 配置文件。
3. 少量日志与配置覆盖相关环境变量。

Relay (中继端) 支持：

1. `-config` 命令行参数。
2. `relay.json` 配置文件。
3. 丰富的启动覆盖环境变量。

Client (内网客户端) 支持：

1. `-config` 命令行参数。
2. `flared.json` 配置文件。
3. 启动覆盖与日志环境变量。

## 配置文件位置

| 组件 | 默认位置 | 说明 |
| --- | --- | --- |
| Server SQLite | `openflare.db` | 可通过 `SQLITE_PATH` 修改 |
| Agent 配置文件 | `./agent.json` | 可通过 `-config` 指定 |
| 一键安装 Agent 配置 | `/opt/openflare-agent/agent.json` | 安装脚本默认生成 |
| Agent 数据目录 | 配置文件所在目录下的 `data` | 可通过 `data_dir` 修改 |
| Relay 配置文件 | `./relay.json` | 可通过 `-config` 指定 |
| 一键安装 Relay 配置 | `/opt/openflare-relay/relay.json` | 安装脚本默认生成 |
| Client 配置文件 | `./flared.json` | 可通过 `-config` 指定 |
| 一键安装 Client 配置 | `/opt/openflared/flared.json` | 安装脚本默认生成 |

## Server 命令行参数

```bash
cd openflare-server
go run . --port 3000 --log-dir ./logs
```

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `--port` | 指定 Server 监听端口 | `3000` |
| `--log-dir` | 指定日志目录 | 空 |
| `--version` | 输出当前版本后退出 | `false` |
| `--help` | 输出帮助信息后退出 | `false` |

## Server 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `PORT` | Server 监听端口 | `3000` |
| `GIN_MODE` | Gin 运行模式 | 非 `debug` 时按 release |
| `LOG_LEVEL` | 日志等级 | `info` |
| `JWT_SECRET` | 管理端 API 登录令牌的 JWT 签名密钥，生产环境必须显式配置 | 启动时随机生成 |
| `SQLITE_PATH` | SQLite 数据库文件路径 | `openflare.db` |
| `DSN` | PostgreSQL DSN，设置后优先于 SQLite | 空 |
| `SQL_DSN` | 兼容旧命名的 PostgreSQL DSN，优先级低于 `DSN` | 空 |
| `REDIS_CONN_STRING` | Redis 连接串 | 空 |
| `AGENT_TOKEN` | 兼容旧部署的全局 Agent Token | 空 |

说明：

* `DSN` 与 `SQL_DSN` 同时存在时优先使用 `DSN`。
* `DSN` 或 `SQL_DSN` 与 `SQLITE_PATH` 同时存在时优先使用 PostgreSQL。
* 当目标 PostgreSQL 数据库为空且本地 `SQLITE_PATH` 文件存在时，Server 启动阶段会自动迁移 SQLite 数据，并在日志中输出按表迁移进度。
* `JWT_SECRET` 用于管理端 API 登录令牌的签名与验证，生产环境必须显式配置，避免重启后所有已登录令牌失效。
* `REDIS_CONN_STRING` 未配置时，相关能力回退为进程内实现。

## 运行时 Option

以下配置由管理端设置页维护，可热更新：

| 配置项 | 作用 | 默认值 |
| --- | --- | --- |
| `AgentHeartbeatInterval` | Agent 心跳间隔（毫秒） | `10000` |
| `AgentWebsocketUpgradeEnabled` | 是否允许 Agent 在 HTTP 心跳成功后升级为 WebSocket | `true` |
| `NodeOfflineThreshold` | 节点离线阈值（毫秒） | `120000` |
| `AgentUpdateRepo` | Agent 自更新仓库 | `Rain-kl/OpenFlare` |
| `GeoIPProvider` | 节点/IP 归属解析方式 | `ipinfo` |
| `DatabaseAutoCleanupEnabled` | 是否启用每日自动清理观测数据 | `false` |
| `DatabaseAutoCleanupRetentionDays` | 自动清理保留天数，至少 1 天 | `30` |
| `UptimeKumaEnabled` | 是否启用 Uptime Kuma 自动同步 | `false` |
| `UptimeKumaUrl` | Uptime Kuma 实例地址 | 空 |
| `UptimeKumaUsername` | Uptime Kuma 登录用户名 | 空 |
| `UptimeKumaPassword` | Uptime Kuma 登录密码（写专，接口不回显） | 空 |
| `UptimeKumaMonitorScope` | 监控范围，支持 `all` (全部站点) 或 `selected` (选择站点) | `all` |
| `UptimeKumaSelectedSites` | 已选择监控站点的名称列表（英文逗号分隔） | 空 |
| `UptimeKumaSyncInterval` | 自动差分同步间隔（分钟） | `5` |
| `UptimeKumaInterval` | 监控心跳检测频率（秒） | `60` |
| `UptimeKumaRetry` | 监控最大重试次数 | `0` |
| `UptimeKumaRetryInterval` | 监控重试间隔时间（秒） | `60` |
| `UptimeKumaTimeout` | 监控请求超时断开时间（秒） | `48` |

说明：

* `DatabaseAutoCleanupEnabled` 开启后，Server 会在每天凌晨 3 点自动清理 `node_access_logs`、`node_metric_snapshots`、`node_request_reports` 三类观测数据。
* `DatabaseAutoCleanupRetentionDays` 为统一保留天数，必须大于等于 1。
* 管理端支持手动清理时留空保留天数，以直接删除对应数据集的全部历史记录。
* `AgentUpdateRepo` 指向的 GitHub Release 必须为每个 Agent 二进制提供同名 `.sha256` 校验文件，例如 `openflare-agent-linux-amd64.sha256`；Agent 自更新会在替换可执行文件前校验 SHA-256。
* 第三方登录不再通过 `GitHubOAuthEnabled`、`GitHubClientId`、`GitHubClientSecret` 作为主配置入口；这些旧 Option 仅用于升级时迁移默认 GitHub 认证源。
* 微信登录旧 Option 保留为兼容字段，但管理端不再提供微信登录配置入口。
* Turnstile 旧 Option 与后端校验能力保留，已有配置仍会生效。

## OpenResty 参数

OpenResty 性能参数与缓存参数继续统一保存在 `Option` 表。当前常用项包括：

* `OpenRestyWorkerProcesses`
* `OpenRestyWorkerConnections`
* `OpenRestyWorkerRlimitNofile`
* `OpenRestyKeepaliveTimeout`
* `OpenRestyProxyConnectTimeout`
* `OpenRestyProxySendTimeout`
* `OpenRestyProxyReadTimeout`
* `OpenRestyProxyBufferingEnabled`
* `OpenRestyGzipEnabled`
* `OpenRestyCacheEnabled`
* `OpenRestyCachePath`
* `OpenRestyCacheMaxSize`

这类参数必须以结构化方式校验、保存并参与版本渲染。

约束：

* 管理端不再暴露 `resolver` 配置。
* 规则上游统一渲染为 named `upstream` 并启用 keepalive。
* 单上游如带 base path 或 query，会在 `proxy_pass` 中补回原始 URI。
* 多上游仍要求每个上游都为纯 `scheme://host[:port]`，且同一规则内协议一致。
* `OpenRestyCacheEnabled` 用于启用缓存基础设施与全局默认参数；实际是否缓存、按 URL / 后缀 / 路径等命中策略由各条 `proxy_routes` 单独决定。
* 默认缓存 Key 为 `$scheme$host$request_uri`。
* 默认 `keepalive_timeout` 为 `20` 秒，默认 `proxy_connect_timeout` 为 `3` 秒。
* 默认事件模型为 `epoll`，并默认开启 `multi_accept`。
* HTTPS 监听默认使用独立 `http2 on;` 指令，避免新版 Nginx/OpenResty 对 `listen ... http2` 的弃用告警。

## 前端构建环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | 前端请求 API 的基础路径 | `/api` |
| `NEXT_PUBLIC_APP_VERSION` | 前端展示版本号 | `dev` |
| `NEXT_DEV_BACKEND_URL` | 本地开发服务器代理的后端地址 | `http://127.0.0.1:3000` |

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

## Agent 命令行参数

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `-config` | 指定 Agent 配置文件路径 | `./agent.json` |

## Agent 配置字段

| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `agent_token` | 节点专属认证 Token | 与 `discovery_token` 二选一 | 空 |
| `discovery_token` | 首次自动注册使用的全局 Token | 与 `agent_token` 二选一 | 空 |
| `node_name` | 节点名称 | 否 | 自动使用主机名 |
| `node_ip` | 节点 IP | 否 | 自动探测，优先通过第三方 API 获取真实出口公网 IP；失败时退回本机网卡探测 |
| `openresty_path` | OpenResty 二进制路径 | 否 | `openresty` |
| `openresty_observability_port` | 本地观测与 OpenResty 健康检查端口 | 否 | `18081` |
| `data_dir` | Agent 数据目录 | 否 | 配置文件所在目录下的 `data` |
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
| `mmdb_update_interval` | WAF GeoIP mmdb 更新间隔 | 否 | `86400000` 毫秒 |
| `mmdb_download_url` | WAF GeoIP mmdb 下载地址 | 否 | 内置 GeoLite2 Country 下载地址 |
| `observability_buffer_path` | 观测补报缓冲文件路径 | 否 | `data_dir/var/lib/openflare/observability-buffer.json` |
| `observability_replay_minutes` | 自动补传最近观测窗口分钟数 | 否 | `15` |
| `state_path` | Agent 本地状态文件路径 | 否 | `data_dir/var/lib/openflare/agent-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒 |

说明：

* `agent_token` 与 `discovery_token` 不能同时为空。
* `heartbeat_interval` 与 `request_timeout` 支持毫秒整数或 Go duration 字符串。
* Server 运行时配置 `AgentWebsocketUpgradeEnabled` 开启时，Agent 会在 HTTP 心跳成功后尝试升级为 WebSocket；连接失败或断开后自动退回 HTTP 心跳。
* 未配置 `openresty_path` 时默认调用 `openresty`。
* Agent 周期性健康检查会请求 `http://127.0.0.1:<openresty_observability_port>/openflare/stub_status`，不再通过高频 `openresty -t` 判断运行时健康；配置应用、启动恢复和 reload 前校验仍会执行 `openresty -t -c <main_config_path>`。
* Agent 会初始化并定期更新 `mmdb_path`，供 OpenResty WAF Lua 执行国家级地域规则；更新失败只记录警告，不阻断同步或 reload。
* 当激活配置引用 Pages 部署时，Agent 会在应用 OpenResty 配置前，将部署包下载、校验并解压到 `pages_dir`，OpenResty 通过该目录服务静态文件。
* 如果 `agent.json` 不存在，但 `OPENFLARE_SERVER_URL` 与 Token 等环境变量足够，Agent 可以直接启动；两者同时存在时环境变量优先。
* Agent 未配置 `node_ip` 时，会优先通过 `https://realip.cc` 获取真实出口公网 IP，适配 Docker/NAT 场景；该请求失败时，才退回本机网卡探测并优先选择公网 IPv4。
* Agent 自动探测到私网 `node_ip` 时，Server 会在注册/心跳阶段优先保留 Agent 直连来源的公网地址，避免 NAT/多网卡场景误登记内网网卡地址。
* 在管理端开启“锁定节点 IP”后，Server 会保留管理端填写的节点 IP，后续 Agent 注册、HTTP 心跳或 WebSocket 状态上报不会覆盖该字段；关闭锁定后，下一次上报可重新回填。

## Relay 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `LOG_LEVEL` | Relay 日志等级 | `info` |
| `OPENFLARE_SERVER_URL` | 控制面地址，可覆盖 `relay.json` | 空 |
| `OPENFLARE_AGENT_TOKEN` | 节点专属认证 Token，可覆盖 `relay.json` | 空 |
| `OPENFLARE_DISCOVERY_TOKEN` | 首次自动注册 Token，可覆盖 `relay.json` | 空 |
| `OPENFLARE_NODE_NAME` | 节点名称，可覆盖 `relay.json` | 空 |
| `OPENFLARE_NODE_IP` | 节点 IP，可覆盖 `relay.json` | 空 |
| `OPENFLARE_DATA_DIR` | Relay 数据目录，可覆盖 `relay.json` | 空 |
| `OPENFLARE_FRPS_PATH` | frps 二进制路径，可覆盖 `relay.json` | 空 |

## Relay 命令行参数

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `-config` | 指定 Relay 配置文件路径 | `./relay.json` |

## Relay 配置字段

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
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒，支持 Go duration 字符串 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒，支持 Go duration 字符串 |

## OpenFlared (Client) 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `LOG_LEVEL` | Client 日志等级 | `info` |
| `OPENFLARE_SERVER_URL` | 控制面地址，可覆盖 `flared.json` | 空 |
| `OPENFLARE_TUNNEL_TOKEN` | 隧道专属认证 Token，可覆盖 `flared.json` | 空 |
| `OPENFLARE_DATA_DIR` | Client 数据目录，可覆盖 `flared.json` | 空 |
| `OPENFLARE_FRPC_PATH` | frpc 二进制路径，可覆盖 `flared.json` | 空 |

## OpenFlared (Client) 命令行参数

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `-config` | 指定 Client 配置文件路径 | `./flared.json` |

## OpenFlared (Client) 配置字段

| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `tunnel_token` | 隧道专属认证 Token | 是 | 无 |
| `frpc_path` | frpc 二进制路径 | 否 | `frpc`（在系统 PATH 中寻找） |
| `data_dir` | Client 运行时数据目录 | 否 | 配置文件所在目录下的 `data` |
| `state_path` | Client 本地状态文件存储路径 | 否 | `data_dir/flared-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒，支持 Go duration 字符串 |
| `sync_interval` | 配置拉取同步间隔 | 否 | `30000` 毫秒，支持 Go duration 字符串 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒，支持 Go duration 字符串 |


## 常见配置组合

### 生产 Server + PostgreSQL

```bash
export JWT_SECRET='replace-with-a-long-random-string'
export DSN='postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable'
export GIN_MODE='release'
export LOG_LEVEL='info'
```

### 本地 Server + SQLite

```bash
export JWT_SECRET='dev-jwt-secret'
export SQLITE_PATH='./openflare-dev.db'
export LOG_LEVEL='debug'
go run .
```

### Agent + 默认 OpenResty

```json
{
  "server_url": "http://your-server:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "/opt/openflare-agent/data",
  "openresty_path": "openresty",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

### Agent + 自定义 OpenResty 路径

```json
{
  "server_url": "http://your-server:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "/var/lib/openflare-agent",
  "openresty_path": "/usr/local/openresty/nginx/sbin/openresty",
  "main_config_path": "/var/lib/openflare-agent/etc/nginx/nginx.conf",
  "route_config_path": "/var/lib/openflare-agent/etc/nginx/conf.d/openflare_routes.conf",
  "access_log_path": "/var/lib/openflare-agent/var/log/openflare/access.log",
  "cert_dir": "/var/lib/openflare-agent/etc/nginx/certs",
  "lua_dir": "/var/lib/openflare-agent/etc/nginx/lua",
  "runtime_config_dir": "/var/lib/openflare-agent/etc/openflare",
  "pages_dir": "/var/lib/openflare-agent/var/lib/openflare/pages",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

### Relay (中继端) 默认配置

`relay.json`：

```json
{
  "server_url": "http://your-server:3000",
  "agent_token": "replace-with-relay-auth-token",
  "frps_path": "frps",
  "data_dir": "/opt/openflare-relay/data",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

### OpenFlared (内网客户端) 默认配置

`flared.json`：

```json
{
  "server_url": "http://your-server:3000",
  "tunnel_token": "replace-with-tunnel-token",
  "frpc_path": "frpc",
  "data_dir": "/opt/openflared/data",
  "heartbeat_interval": 10000,
  "sync_interval": 30000,
  "request_timeout": 10000
}
```


## 维护要求

以下内容变化时，必须同步更新本文档：

* Server 命令行参数。
* Server 环境变量。
* Agent 命令行参数与配置字段。
* Relay 命令行参数与配置字段。
* Client 命令行参数与配置字段。
* 任一配置项的默认值、用途或示例。
