# OpenFlare 配置项说明

本文档汇总 OpenFlare `1.0.0` 当前支持的 Server 与 Agent 配置项，只保留仍然有效的启动、部署与运行参数。

## 1. Server 配置

Server 支持三类配置来源：

1. 命令行参数
2. 环境变量
3. 数据库 `Option` 表中的运行时配置

### 1.1 命令行参数

```bash
cd openflare_server
go run . --port 3000 --log-dir ./logs
```

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `--port` | 指定 Server 监听端口 | `3000` |
| `--log-dir` | 指定日志目录 | 空 |
| `--version` | 输出当前版本后退出 | `false` |
| `--help` | 输出帮助信息后退出 | `false` |

### 1.2 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `PORT` | Server 监听端口 | `3000` |
| `GIN_MODE` | Gin 运行模式 | 非 `debug` 时按 release |
| `LOG_LEVEL` | 日志等级 | `info` |
| `SESSION_SECRET` | Session 签名密钥 | 启动时随机生成 |
| `SQLITE_PATH` | SQLite 数据库文件路径 | `openflare.db` |
| `DSN` | PostgreSQL DSN，设置后优先于 SQLite | 空 |
| `SQL_DSN` | 兼容旧命名的 PostgreSQL DSN，优先级低于 `DSN` | 空 |
| `REDIS_CONN_STRING` | Redis 连接串 | 空 |
| `UPLOAD_PATH` | 上传目录 | `upload` |
| `AGENT_TOKEN` | 兼容旧部署的全局 Agent Token | 空 |

说明：

* `DSN` 与 `SQL_DSN` 同时存在时优先使用 `DSN`
* `DSN` 或 `SQL_DSN` 与 `SQLITE_PATH` 同时存在时优先使用 PostgreSQL
* 当目标 PostgreSQL 数据库为空且本地 `SQLITE_PATH` 文件存在时，Server 启动阶段会自动迁移 SQLite 数据，并在日志中输出按表迁移进度
* `SESSION_SECRET` 生产环境必须显式配置
* `REDIS_CONN_STRING` 未配置时，相关能力回退为进程内实现

### 1.3 `Option` 表中的运行时配置

以下配置由管理端设置页维护，可热更新：

| 配置项 | 作用 | 默认值 |
| --- | --- | --- |
| `AgentHeartbeatInterval` | Agent 心跳间隔（毫秒） | `10000` |
| `NodeOfflineThreshold` | 节点离线阈值（毫秒） | `120000` |
| `AgentUpdateRepo` | Agent 自更新仓库 | `Rain-kl/OpenFlare` |
| `GeoIPProvider` | 节点/IP 归属解析方式 | `ipinfo` |
| `GlobalApiRateLimitNum` / `GlobalApiRateLimitDuration` | 全局 API 限流次数 / 时间窗口 | `300` / `180` |
| `GlobalWebRateLimitNum` / `GlobalWebRateLimitDuration` | 全局 Web 限流次数 / 时间窗口 | `300` / `180` |
| `UploadRateLimitNum` / `UploadRateLimitDuration` | 上传接口限流次数 / 时间窗口 | `50` / `60` |
| `DownloadRateLimitNum` / `DownloadRateLimitDuration` | 下载接口限流次数 / 时间窗口 | `50` / `60` |
| `CriticalRateLimitNum` / `CriticalRateLimitDuration` | 敏感接口限流次数 / 时间窗口 | `100` / `1200` |

### 1.4 OpenResty 参数

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
* `OpenRestyResolvers`
* `OpenRestyCacheEnabled`
* `OpenRestyCachePath`
* `OpenRestyCacheMaxSize`

这类参数必须以结构化方式校验、保存并参与版本渲染。

* `OpenRestyResolvers` 由管理端性能页面维护，支持填写多个 DNS 服务器 IP；留空时不额外生成 `resolver` 指令。
* `OpenRestyCacheEnabled` 用于启用缓存基础设施与全局默认参数；实际是否缓存、按 URL / 后缀 / 路径等命中策略由各条 `proxy_routes` 单独决定，不再默认对所有规则开启缓存。
* 默认缓存 Key 为 `$scheme$host$request_uri`，更贴近代理域名维度；如需按其他维度命中，可在性能页显式覆盖。
* 默认 `keepalive_timeout` 为 `20` 秒，默认 `proxy_connect_timeout` 为 `3` 秒，优先兼顾资源占用与回源失败切换速度。
* 默认事件模型为 `epoll`，并默认开启 `multi_accept`；HTTPS 监听默认附带 `reuseport`，以改善多 worker 下的连接分发。
### 1.5 前端构建环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | 前端请求 API 的基础路径 | `/api` |
| `NEXT_PUBLIC_APP_VERSION` | 前端展示版本号 | `dev` |
| `NEXT_DEV_BACKEND_URL` | 本地开发服务器代理的后端地址 | `http://127.0.0.1:3000` |

## 2. Agent 配置

Agent 当前支持：

1. `-config` 命令行参数
2. `agent.json` 配置文件
3. 少量日志相关环境变量

### 2.1 Agent 环境变量

| 环境变量 | 作用 | 默认值 |
| --- | --- | --- |
| `LOG_LEVEL` | Agent 日志等级 | `info` |

### 2.2 Agent 命令行参数

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `-config` | 指定 Agent 配置文件路径 | `./agent.json` |

### 2.3 Agent 配置字段

| 字段 | 作用 | 是否必填 | 默认值/行为 |
| --- | --- | --- | --- |
| `server_url` | 控制面地址 | 是 | 无 |
| `agent_token` | 节点专属认证 Token | 与 `discovery_token` 二选一 | 空 |
| `discovery_token` | 首次自动注册使用的全局 Token | 与 `agent_token` 二选一 | 空 |
| `node_name` | 节点名称 | 否 | 自动使用主机名 |
| `node_ip` | 节点 IP | 否 | 自动探测 |
| `openresty_path` | 本机 OpenResty 路径 | 否 | 空，未设置时走 Docker 模式 |
| `openresty_container_name` | Docker 模式下的容器名 | 否 | `openflare-openresty` |
| `openresty_docker_image` | Docker 模式下的镜像 | 否 | `openresty/openresty:alpine` |
| `openresty_observability_port` | 本地观测端口 | 否 | `18081` |
| `docker_binary` | Docker 可执行文件名或路径 | 否 | `docker` |
| `data_dir` | Agent 数据目录 | 否 | 配置文件所在目录下的 `data` |
| `main_config_path` | OpenResty 主配置写入路径 | 否 | 本机模式建议显式配置 |
| `route_config_path` | 路由配置写入路径 | 否 | `data_dir/etc/nginx/conf.d/openflare_routes.conf` |
| `cert_dir` | 本机证书写入目录 | 否 | `data_dir/etc/nginx/certs` |
| `openresty_cert_dir` | OpenResty 读取证书目录 | 否 | 随运行模式变化 |
| `lua_dir` | 本机 Lua 脚本写入目录 | 否 | `data_dir/etc/nginx/lua` |
| `openresty_lua_dir` | OpenResty 读取 Lua 目录 | 否 | 随运行模式变化 |
| `observability_buffer_path` | 观测补报缓冲文件路径 | 否 | `data_dir/var/lib/openflare/observability-buffer.json` |
| `observability_replay_minutes` | 自动补传最近观测窗口分钟数 | 否 | `15` |
| `state_path` | Agent 本地状态文件路径 | 否 | `data_dir/var/lib/openflare/agent-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒 |
| `request_timeout` | HTTP 请求超时 | 否 | `10000` 毫秒 |

说明：

* `agent_token` 与 `discovery_token` 不能同时为空
* `heartbeat_interval` 与 `request_timeout` 支持毫秒整数或 Go duration 字符串
* 未配置 `openresty_path` 时默认使用 Docker OpenResty 模式

## 3. 维护要求

以下内容变化时，必须同步更新本文档：

* Server 命令行参数
* Server 环境变量
* Agent 命令行参数
* Agent 配置字段
* 任一配置项的默认值、用途或示例
