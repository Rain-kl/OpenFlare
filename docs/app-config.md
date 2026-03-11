# ATSFlare 配置项说明

本文档汇总当前 ATSFlare Server 与 Agent 在启动、部署和运行时支持的参数、环境变量与配置文件字段，并说明其作用、默认值和示例。

---

## 1. Server 配置

Server 当前支持两类启动配置：

1. 命令行参数
2. 环境变量

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
| `SESSION_SECRET` | Session 签名密钥；生产环境必须显式设置，避免重启后会话失效 | 启动时随机生成 UUID | `SESSION_SECRET=replace-with-random-string` |
| `SQLITE_PATH` | SQLite 数据库文件路径 | `atsflare.db` | `SQLITE_PATH=/data/atsflare.db` |
| `SQL_DSN` | MySQL DSN；设置后优先使用 MySQL，而不是 SQLite | 未设置时使用 SQLite | `SQL_DSN=user:pass@tcp(127.0.0.1:3306)/atsflare` |
| `REDIS_CONN_STRING` | Redis 连接串；设置后启用 Redis，用于 Session/限流相关能力 | 未设置时关闭 Redis | `REDIS_CONN_STRING=redis://default:pass@127.0.0.1:6379/0` |
| `UPLOAD_PATH` | 上传文件目录 | `upload` | `UPLOAD_PATH=/data/upload` |
| `AGENT_TOKEN` | 全局 Agent Token 兼容配置；当前默认部署不依赖该变量 | 空 | `AGENT_TOKEN=legacy-shared-token` |

说明：

* `SQL_DSN` 与 `SQLITE_PATH` 同时存在时，优先使用 `SQL_DSN`
* `SESSION_SECRET` 未固定时，每次重启都会生成新的随机值，已登录用户的 Cookie 会失效
* `REDIS_CONN_STRING` 未配置时，相关能力将回退为进程内实现
* `UPLOAD_PATH` 目录在启动时若不存在会自动创建

---

## 2. Agent 配置

Agent 当前支持两类启动配置：

1. 命令行参数
2. `agent.json` 配置文件

当前 Agent **没有额外环境变量作为正式配置入口**，启动行为主要由 `-config` 参数和配置文件字段决定。

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
	"nginx_container_name": "atsflare-nginx",
	"nginx_docker_image": "nginx:stable-alpine",
	"heartbeat_interval": 30000,
	"sync_interval": 30000,
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
	"nginx_path": "/usr/sbin/nginx",
	"route_config_path": "/etc/nginx/conf.d/atsflare_routes.conf",
	"cert_dir": "/etc/nginx/certs",
	"nginx_cert_dir": "/etc/nginx/certs",
	"state_path": "./data/agent-state.json",
	"heartbeat_interval": 30000,
	"sync_interval": 30000,
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
| `nginx_path` | 本机 Nginx 可执行文件路径；设置后按本机 Nginx 模式运行 | 否 | 空；未设置时按 Docker Nginx 模式处理 | `/usr/sbin/nginx` |
| `nginx_container_name` | Docker 模式下的 Nginx 容器名 | 否 | `atsflare-nginx` | `atsflare-nginx` |
| `nginx_docker_image` | Docker 模式下用于初始化/管理的 Nginx 镜像 | 否 | `nginx:stable-alpine` | `nginx:stable-alpine` |
| `docker_binary` | Docker 可执行文件名或路径 | 否 | `docker` | `/usr/bin/docker` |
| `data_dir` | Agent 数据目录，用于存储托管配置、证书和状态文件 | 否 | 配置文件所在目录下的 `data` 子目录 | `./data` |
| `route_config_path` | 路由配置文件写入路径 | 否 | 默认为 `data_dir` 下托管路径 | `/etc/nginx/conf.d/atsflare_routes.conf` |
| `cert_dir` | Agent 在本机写入证书文件的目录 | 否 | 默认为 `data_dir` 下托管证书目录 | `./data/etc/nginx/certs` |
| `nginx_cert_dir` | Nginx 实际读取证书的目录 | 否 | 本机模式默认等于 `cert_dir`；Docker 模式默认 `/etc/nginx/atsflare-certs` | `/etc/nginx/certs` |
| `state_path` | Agent 本地状态文件路径 | 否 | 默认为 `data_dir` 下托管状态文件 | `./data/agent-state.json` |
| `heartbeat_interval` | 心跳间隔 | 否 | `30000` 毫秒 | `30000` |
| `sync_interval` | 配置同步间隔 | 否 | `30000` 毫秒 | `30000` |
| `request_timeout` | HTTP 请求超时时间 | 否 | `10000` 毫秒 | `10000` |

说明：

* `agent_token` 与 `discovery_token` 不能同时为空
* `heartbeat_interval`、`sync_interval`、`request_timeout` 支持两种写法：
	* 毫秒整数，例如 `30000`
	* Go duration 字符串，例如 `"30s"`
* `node_name` 与 `node_ip` 未填写时会自动探测；若自动探测失败，配置校验会报错
* 未配置 `nginx_path` 时，默认为 Docker Nginx 模式
* 配置保存时，`agent_version`、`nginx_version` 由程序运行时维护，不需要写入 JSON

### 2.4 Agent 托管路径默认值

当未显式设置以下字段时，Agent 会根据 `data_dir` 自动生成托管路径：

| 字段 | 默认值 |
| --- | --- |
| `route_config_path` | `data_dir/etc/nginx/conf.d/atsflare_routes.conf` |
| `cert_dir` | `data_dir/etc/nginx/certs` |
| `state_path` | `data_dir/var/lib/atsflare/agent-state.json` |

Docker Nginx 模式下：

| 字段 | 默认值 |
| --- | --- |
| `nginx_cert_dir` | `/etc/nginx/atsflare-certs` |

### 2.5 Agent 启动示例

#### Docker Nginx 模式

适用于节点本机不直接管理宿主机 Nginx，而是通过 Docker 容器运行 Nginx。

```json
{
	"server_url": "http://127.0.0.1:3000",
	"discovery_token": "replace-with-global-discovery-token",
	"data_dir": "./data"
}
```

#### 本机 Nginx 模式

适用于节点已经安装了宿主机 Nginx，且 Agent 直接执行 `nginx -t` 与 `nginx -s reload`。

```json
{
	"server_url": "http://127.0.0.1:3000",
	"agent_token": "replace-with-node-auth-token",
	"nginx_path": "/usr/sbin/nginx",
	"route_config_path": "/etc/nginx/conf.d/atsflare_routes.conf",
	"cert_dir": "/etc/nginx/certs",
	"nginx_cert_dir": "/etc/nginx/certs"
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
