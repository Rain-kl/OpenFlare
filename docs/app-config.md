# ATSFlare 配置项说明

本文档汇总当前 ATSFlare Server 与 Agent 在启动、部署和运行时支持的参数、环境变量与配置文件字段，并说明其作用、默认值和示例。

---

## 1. Server 配置

Server 当前支持两类启动配置：

1. 命令行参数
2. 环境变量

此外，部分运行时参数已迁入数据库 `Option` 表，可在管理端设置页中热更新，例如 Agent 运行参数与限流阈值。

第五版（0.5.x）继续复用 `Option` 表承载 OpenResty 性能优化参数与缓存参数，不单独引入新的配置中心。

### 1.1 Server 命令行参数

启动示例：

```bash
cd atsf_server
go run . --port 3000 --log-dir ./logs
```

或在编译后二进制中使用：

```bash
./atsflare --port 3000 --log-dir ./logs
```

支持的命令行参数：

| 参数 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `--port` | 指定 Server 监听端口 | `3000` | `--port 3000` |
| `--log-dir` | 指定日志目录；设置后会自动创建目录并写入日志 | 空，默认输出到 stdout | `--log-dir ./logs` |
| `--version` | 输出当前版本后退出 | `false` | `./atsflare --version` |
| `--help` | 输出帮助信息后退出 | `false` | `./atsflare --help` |

说明：

* 当同时设置 `PORT` 环境变量与 `--port` 时，运行时优先使用 `PORT`
* `--log-dir` 当前没有对应环境变量，适合源码运行或 systemd 方式部署时使用

### 1.2 Server 环境变量

源码启动示例：

```bash
cd atsf_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./atsflare.db'
export GIN_MODE='release'
export LOG_LEVEL='info'
export PORT='3000'
go run .
```

Docker Compose 示例：

```yaml
services:
	atsflare:
		image: ghcr.io/rain-kl/atsflare:latest
		restart: unless-stopped
		ports:
			- "3000:3000"
		environment:
			SESSION_SECRET: replace-with-random-string
			SQLITE_PATH: /data/atsflare.db
			GIN_MODE: release
			LOG_LEVEL: info
			PORT: "3000"
		volumes:
			- atsflare-data:/data

volumes:
	atsflare-data:
```

支持的环境变量：

| 环境变量 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `PORT` | 指定 Server 实际监听端口 | `3000` | `PORT=3000` |
| `GIN_MODE` | 指定 Gin 运行模式；仅当值为 `debug` 时启用 debug，其余情况按 release 运行 | 非 `debug` 默认按 release 运行 | `GIN_MODE=release` |
| `LOG_LEVEL` | 指定系统日志等级；支持 `debug`/`info`/`warn`/`error`（`warning` 兼容为 `warn`） | `info` | `LOG_LEVEL=warn` |
| `SESSION_SECRET` | Session 签名密钥；生产环境必须显式设置，避免重启后会话失效 | 启动时随机生成 UUID | `SESSION_SECRET=replace-with-random-string` |
| `SQLITE_PATH` | SQLite 数据库文件路径 | `atsflare.db` | `SQLITE_PATH=/data/atsflare.db` |
| `SQL_DSN` | MySQL DSN；设置后优先使用 MySQL，而不是 SQLite | 未设置时使用 SQLite | `SQL_DSN=user:pass@tcp(127.0.0.1:3306)/atsflare` |
| `REDIS_CONN_STRING` | Redis 连接串；设置后启用 Redis，用于 Session/限流相关能力 | 未设置时关闭 Redis | `REDIS_CONN_STRING=redis://default:pass@127.0.0.1:6379/0` |
| `UPLOAD_PATH` | 上传文件目录 | `upload` | `UPLOAD_PATH=/data/upload` |
| `AGENT_TOKEN` | 全局 Agent Token 兼容配置；当前默认部署不依赖该变量 | 空 | `AGENT_TOKEN=legacy-shared-token` |

说明：

* `SQL_DSN` 与 `SQLITE_PATH` 同时存在时，优先使用 `SQL_DSN`
* `SESSION_SECRET` 未固定时，每次重启都会生成新的随机值，已登录用户的 Cookie 会失效
* `LOG_LEVEL` 未设置或设置为不支持的值时，将回退为 `info`
* `REDIS_CONN_STRING` 未配置时，相关能力将回退为进程内实现
* `UPLOAD_PATH` 目录在启动时若不存在会自动创建

### 1.2.1 设置页可热更新的运行时配置

以下配置不依赖环境变量，保存在数据库 `Option` 表中，可在管理端设置页的「运维设置」中调整，并在保存后立即生效：

| 配置项 | 作用 | 默认值 |
| --- | --- | --- |
| `AgentHeartbeatInterval` | Agent 心跳间隔（毫秒） | `10000` |
| `NodeOfflineThreshold` | 节点离线判定阈值（毫秒） | `120000` |
| `AgentUpdateRepo` | Agent 自更新仓库 | `Rain-kl/ATSFlare` |
| `GeoIPProvider` | IP 归属解析方式；支持 `disabled`、`mmdb`、`ip-api`、`geojs`、`ipinfo` | `disabled` |
| `GlobalApiRateLimitNum` / `GlobalApiRateLimitDuration` | 全局 API 限流次数 / 时间窗口（秒） | `300` / `180` |
| `GlobalWebRateLimitNum` / `GlobalWebRateLimitDuration` | 全局 Web 限流次数 / 时间窗口（秒） | `300` / `180` |
| `UploadRateLimitNum` / `UploadRateLimitDuration` | 上传接口限流次数 / 时间窗口（秒） | `50` / `60` |
| `DownloadRateLimitNum` / `DownloadRateLimitDuration` | 下载接口限流次数 / 时间窗口（秒） | `50` / `60` |
| `CriticalRateLimitNum` / `CriticalRateLimitDuration` | 登录、注册、验证码等敏感接口限流次数 / 时间窗口（秒） | `100` / `1200` |

说明：

* 限流窗口上限不能超过 `RateLimitKeyExpirationDuration`，当前为 20 分钟
* 限流按来源 IP 统计，若前置了 Nginx/CDN/LB，应正确透传真实客户端 IP
* `GeoIPProvider=mmdb` 时，Server 会按需下载并使用本地 MaxMind Country 数据库；其余非 `disabled` 选项会直接请求对应外部 GeoIP 服务

### 1.2.2 第五版当前支持的 OpenResty 优化配置项

以下配置项当前已接入管理端「运维设置」，并统一保存在 `Option` 表中。它们会参与第五版后续的配置渲染与发布链路；当前阶段优先完成参数录入、默认值和校验能力。

| 配置项 | 作用 | 计划默认值 |
| --- | --- | --- |
| `OpenRestyWorkerProcesses` | `worker_processes` 配置；支持 `auto` 或正整数 | `auto` |
| `OpenRestyWorkerConnections` | `events { worker_connections }` 上限 | `4096` |
| `OpenRestyWorkerRlimitNofile` | `worker_rlimit_nofile` 上限 | `65535` |
| `OpenRestyEventsUse` | `events { use ... }` 指令；为空表示不显式渲染 | 空 |
| `OpenRestyEventsMultiAcceptEnabled` | 是否启用 `multi_accept on` | `false` |
| `OpenRestyKeepaliveTimeout` | `keepalive_timeout` 秒数 | `65` |
| `OpenRestyKeepaliveRequests` | `keepalive_requests` 上限 | `1000` |
| `OpenRestyClientHeaderTimeout` | `client_header_timeout` 秒数 | `15` |
| `OpenRestyClientBodyTimeout` | `client_body_timeout` 秒数 | `15` |
| `OpenRestySendTimeout` | `send_timeout` 秒数 | `30` |
| `OpenRestyProxyConnectTimeout` | `proxy_connect_timeout` 秒数 | `5` |
| `OpenRestyProxySendTimeout` | `proxy_send_timeout` 秒数 | `60` |
| `OpenRestyProxyReadTimeout` | `proxy_read_timeout` 秒数 | `60` |
| `OpenRestyProxyRequestBufferingEnabled` | 是否启用 `proxy_request_buffering` | `false` |
| `OpenRestyProxyBufferingEnabled` | 是否启用 `proxy_buffering` | `true` |
| `OpenRestyProxyBuffers` | `proxy_buffers` 组合值，例如 `16 16k` | `16 16k` |
| `OpenRestyProxyBufferSize` | `proxy_buffer_size` | `8k` |
| `OpenRestyProxyBusyBuffersSize` | `proxy_busy_buffers_size` | `64k` |
| `OpenRestyGzipEnabled` | 是否启用 `gzip on` | `true` |
| `OpenRestyGzipMinLength` | `gzip_min_length` 字节数 | `1024` |
| `OpenRestyGzipCompLevel` | `gzip_comp_level` | `5` |
| `OpenRestyCacheEnabled` | 是否启用代理缓存 | `false` |
| `OpenRestyCachePath` | `proxy_cache_path` 目录 | 空 |
| `OpenRestyCacheLevels` | `proxy_cache_path levels=` 值 | `1:2` |
| `OpenRestyCacheInactive` | `proxy_cache_path inactive=` 时长 | `30m` |
| `OpenRestyCacheMaxSize` | `proxy_cache_path max_size=` 大小 | `1g` |
| `OpenRestyCacheKeyTemplate` | 缓存 Key 模板 | `$scheme$proxy_host$request_uri` |
| `OpenRestyCacheLockEnabled` | 是否启用 `proxy_cache_lock` | `true` |
| `OpenRestyCacheLockTimeout` | `proxy_cache_lock_timeout` 时长 | `5s` |
| `OpenRestyCacheUseStale` | `proxy_cache_use_stale` 场景列表 | `error timeout updating http_500 http_502 http_503 http_504` |

说明：

* 第五版第一批当前仅开放稳定、可校验、可回滚的常用性能项；更多指令后续按相同模式扩展
* 所有大小、时长、布尔和整数参数都应在 Server 保存前完成校验
* `OpenRestyCacheEnabled=false` 时，缓存目录与缓存参数应允许留空或回退到默认值
* 任何包含路径的配置项都必须在 Agent 落盘前再次校验可写性与安全边界

### 1.3 前端构建环境变量

新版管理端位于 `atsf_server/web`，构建时支持以下公开环境变量：

| 环境变量 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | 前端请求后端 API 的基础路径；默认走同源 `/api` | `/api` | `NEXT_PUBLIC_API_BASE_URL=https://demo.example.com/api` |
| `NEXT_PUBLIC_APP_VERSION` | 构建时注入前端展示版本号 | `dev` | `NEXT_PUBLIC_APP_VERSION=v0.4.0` |

说明：

* 以上变量在前端构建阶段读取，并会被打包进静态资源
* 推荐生产环境继续使用同源部署，优先保持 `NEXT_PUBLIC_API_BASE_URL=/api`

---

## 2. Agent 配置

Agent 当前支持两类启动配置：

1. 命令行参数
2. `agent.json` 配置文件

当前 Agent 仅支持少量环境变量用于日志输出控制；核心启动行为仍由 `-config` 参数和配置文件字段决定。

### 2.1.1 Agent 环境变量

支持的环境变量：

| 环境变量 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `LOG_LEVEL` | 指定 Agent 的 `slog` 日志等级；支持 `debug`/`info`/`warn`/`error`（`warning` 兼容为 `warn`） | `info` | `LOG_LEVEL=debug` |

说明：

* `LOG_LEVEL` 只影响 Agent 本地日志输出，不改变心跳、同步或配置行为
* 未设置或设置为不支持的值时，将回退为 `info`

### 2.1 Agent 命令行参数

启动示例：

```bash
cd atsf_agent
go run ./cmd/agent -config ./agent.json
```

或编译后二进制：

```bash
./atsflare-agent -config /path/to/agent.json
```

支持的命令行参数：

| 参数 | 作用 | 默认值 | 示例 |
| --- | --- | --- | --- |
| `-config` | 指定 Agent 配置文件路径 | `./agent.json` | `-config /etc/atsflare/agent.json` |

### 2.2 Agent 配置文件示例

推荐最小配置：

```json
{
	"server_url": "http://127.0.0.1:3000",
	"discovery_token": "replace-with-global-discovery-token",
	"data_dir": "./data",
	"openresty_container_name": "atsflare-openresty",
	"openresty_docker_image": "openresty/openresty:alpine",
	"heartbeat_interval": 10000,
	"request_timeout": 10000
}
```

使用节点专属 Token 的示例：

```json
{
	"server_url": "http://127.0.0.1:3000",
	"agent_token": "replace-with-node-auth-token",
	"node_name": "node-01",
	"node_ip": "192.168.1.20",
	"data_dir": "./data",
	"openresty_path": "/usr/local/openresty/nginx/sbin/openresty",
	"main_config_path": "/usr/local/openresty/nginx/conf/nginx.conf",
	"route_config_path": "/usr/local/openresty/nginx/conf/conf.d/atsflare_routes.conf",
	"cert_dir": "/usr/local/openresty/nginx/conf/certs",
	"openresty_cert_dir": "/usr/local/openresty/nginx/conf/certs",
	"state_path": "./data/agent-state.json",
	"heartbeat_interval": 10000,
	"request_timeout": 10000
}
```

### 2.3 Agent 配置字段

| 字段 | 作用 | 是否必填 | 默认值/行为 | 示例 |
| --- | --- | --- | --- | --- |
| `server_url` | 控制面地址，Agent 所有注册、心跳、同步请求都会发往这里 | 是 | 无 | `http://127.0.0.1:3000` |
| `agent_token` | 节点专属认证 Token | 与 `discovery_token` 二选一 | 空 | `node-token-xxx` |
| `discovery_token` | 全局发现 Token，用于节点首次自动注册 | 与 `agent_token` 二选一 | 空 | `discovery-token-xxx` |
| `node_name` | 节点名称 | 否 | 自动使用主机名 | `node-01` |
| `node_ip` | 节点 IP | 否 | 自动探测第一个可用 IPv4 | `192.168.1.20` |
| `openresty_path` | 本机 OpenResty 可执行文件路径；设置后按本机 OpenResty 模式运行 | 否 | 空；未设置时按 Docker OpenResty 模式处理 | `/usr/local/openresty/nginx/sbin/openresty` |
| `openresty_container_name` | Docker 模式下的 OpenResty 容器名 | 否 | `atsflare-openresty` | `atsflare-openresty` |
| `openresty_docker_image` | Docker 模式下用于初始化/管理的 OpenResty 镜像 | 否 | `openresty/openresty:alpine` | `openresty/openresty:alpine` |
| `docker_binary` | Docker 可执行文件名或路径 | 否 | `docker` | `/usr/bin/docker` |
| `data_dir` | Agent 数据目录，用于存储托管配置、证书和状态文件 | 否 | 配置文件所在目录下的 `data` 子目录 | `./data` |
| `main_config_path` | 第五版主配置接管时 OpenResty 主配置文件写入路径 | 第五版本机模式建议必填 | Docker 模式可使用受管默认路径；本机模式建议显式设置 | `/usr/local/openresty/nginx/conf/nginx.conf` |
| `route_config_path` | 路由配置文件写入路径 | 否 | 默认为 `data_dir` 下托管路径 | `/etc/nginx/conf.d/atsflare_routes.conf` |
| `cert_dir` | Agent 在本机写入证书文件的目录 | 否 | 默认为 `data_dir` 下托管证书目录 | `./data/etc/nginx/certs` |
| `openresty_cert_dir` | OpenResty 实际读取证书的目录 | 否 | 本机模式默认等于 `cert_dir`；Docker 模式默认 `/etc/nginx/atsflare-certs` | `/usr/local/openresty/nginx/conf/certs` |
| `state_path` | Agent 本地状态文件路径 | 否 | 默认为 `data_dir` 下托管状态文件 | `./data/agent-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `10000` 毫秒 | `10000` |
| `request_timeout` | HTTP 请求超时时间 | 否 | `10000` 毫秒 | `10000` |

说明：

* `agent_token` 与 `discovery_token` 不能同时为空
* `heartbeat_interval`、`request_timeout` 支持两种写法：
	* 毫秒整数，例如 `10000`
	* Go duration 字符串，例如 `"30s"`
* `node_name` 与 `node_ip` 未填写时会自动探测；若自动探测失败，配置校验会报错
* 未配置 `openresty_path` 时，默认为 Docker OpenResty 模式
* 配置保存时，`agent_version`、`nginx_version` 由程序运行时维护，不需要写入 JSON
* 第五版主配置接管完成后，本机模式下应优先通过 `main_config_path` 由 Agent 写入受管主配置，而不是依赖节点手工维护 include 规则

### 2.4 Agent 托管路径默认值

当未显式设置以下字段时，Agent 会根据 `data_dir` 自动生成托管路径：

| 字段 | 默认值 |
| --- | --- |
| `main_config_path` | 第五版 Docker 模式默认可落在 `data_dir/etc/nginx/nginx.conf`；本机模式建议显式配置 |
| `route_config_path` | `data_dir/etc/nginx/conf.d/atsflare_routes.conf` |
| `cert_dir` | `data_dir/etc/nginx/certs` |
| `state_path` | `data_dir/var/lib/atsflare/agent-state.json` |

Docker OpenResty 模式下：

| 字段 | 默认值 |
| --- | --- |
| `openresty_cert_dir` | `/etc/nginx/atsflare-certs` |

### 2.5 Agent 启动示例

#### Docker OpenResty 模式

适用于节点本机不直接管理宿主机 OpenResty，而是通过 Docker 容器运行 OpenResty。

```json
{
	"server_url": "http://127.0.0.1:3000",
	"discovery_token": "replace-with-global-discovery-token",
	"data_dir": "./data"
}
```

#### 本机 OpenResty 模式

适用于节点已经安装了宿主机 OpenResty，且 Agent 直接执行 `openresty -t` 与 `openresty -s reload`。

```json
{
	"server_url": "http://127.0.0.1:3000",
	"agent_token": "replace-with-node-auth-token",
	"openresty_path": "/usr/local/openresty/nginx/sbin/openresty",
	"main_config_path": "/usr/local/openresty/nginx/conf/nginx.conf",
	"route_config_path": "/usr/local/openresty/nginx/conf/conf.d/atsflare_routes.conf",
	"cert_dir": "/usr/local/openresty/nginx/conf/certs",
	"openresty_cert_dir": "/usr/local/openresty/nginx/conf/certs"
}
```

---

## 3. 配置维护要求

当以下内容发生变化时，应同步更新本文档：

* Server 新增/删除命令行参数
* Server 新增/删除环境变量
* Agent 新增/删除命令行参数
* Agent 新增/删除配置文件字段
* 任一配置项的默认值、示例或用途发生变化
