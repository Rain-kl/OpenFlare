# 部署说明

你会学到：OpenFlare 的推荐部署方式、Server 与 Agent 的运行要求、源码启动方式、联调步骤、升级与卸载入口。

生产环境建议使用 PostgreSQL 作为 Server 数据库，并通过 `config.yaml` 或环境变量配置 `APP_SESSION_SECRET` 等参数。完整 Docker Compose 部署还需 Redis 与 ClickHouse（见仓库根目录 `docker-compose.yaml`）。Agent 部署方式推荐为 Docker 部署（即直接使用内置 OpenResty 的 Agent 镜像）；亦支持通过安装脚本或手动本地运行。

## 部署拓扑

### 标准反代流量路径

```text
Browser
  |
  v
OpenFlare Server :3000
  |
  | Agent API / heartbeat / config pull
  v
OpenFlare Agent
  |
  v
OpenResty binary
  |
  v
Origin service
```

### 内网穿透流量路径

```text
Browser
  |
  v
OpenResty (Agent, WAF/HTTPS 终结)      <-- TunnelRelay 节点
  |
  | proxy_pass (127.0.0.1:{vhost_port})
  v
OpenFlareRelay (frps 进程)              <-- TunnelRelay 节点
  |
  | frp 隧道协议
  v
OpenFlared (frpc 客户端)                <-- 内网服务器
  |
  v
Internal Service (192.168.x.x)
```

## 前置条件

Server：

| 项目 | 要求 |
| --- | --- |
| Go | `1.25+`，仅源码运行需要 |
| Node.js | `18+`，仅源码构建管理端需要 |
| 数据库 | 可写 SQLite 文件目录，或可访问的 PostgreSQL 实例 |
| 端口 | 默认监听 `3000` |

Agent：

| 项目 | 要求 |
| --- | --- |
| 系统 | 安装脚本支持 Linux 和 macOS；systemd 服务仅在 Linux + systemd 环境创建 |
| 架构 | `amd64` 或 `arm64` |
| OpenResty | 本地部署需要可执行 `openresty`，或通过 `--openresty-path` 指定路径 |
| Docker | 仅 Docker 部署 Agent 镜像时需要 |
| 网络 | Agent 节点必须能访问 Server 地址 |
| GeoIP | WAF 地域规则使用 Agent 本地 MaxMind mmdb；Agent 内置初始库并会定期更新 |

### 硬件配置推荐

| 组件 | 最低硬件配额 | 推荐硬件配额 | 说明 |
| --- | --- | --- | --- |
| **Server 控制面** | 1 核 CPU / 1 GB 内存 / 10 GB 磁盘 | 2 核 CPU / 4 GB 内存 / 50 GB+ 磁盘 | 磁盘用量需根据访问日志留存时长与并发流量合理扩容 |
| **Agent 数据面** | 1 核 CPU / 512 MB 内存 / 2 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 10 GB+ 磁盘 | 根据 OpenResty 的并发代理连接量与 WAF 拦截处理扩容 |
| **Relay 中继节点**| 1 核 CPU / 1 GB 内存 / 5 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 20 GB 磁盘 | frps 传输中继吞吐量主要受带宽与 CPU 吞吐能力限制 |
| **OpenFlared 客户端**| 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 1 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 独立运行于内网，自身资源占用极小，保障网络吞吐即可 |

## Docker Compose 部署 Server

仓库根目录已提供完整 `docker-compose.yaml`（含 PostgreSQL、Redis、ClickHouse、Jaeger）。

```bash
cp .env.example .env
# 编辑 .env，至少修改 APP_SESSION_SECRET 与数据库密码
docker compose up -d
docker compose ps
docker compose logs -f wavelet
```

首次访问 `http://localhost:3000`，默认账号为 `root` / `123456`。登录后请立即修改默认密码。

## 源码启动 Server

先构建管理端前端：

```bash
cd frontend
corepack enable
pnpm install
pnpm build:embed
```

再启动 Server（仓库根目录）：

```bash
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
# 可选：使用 PostgreSQL
# export DB_HOST=127.0.0.1 DB_USERNAME=postgres DB_PASSWORD=postgres DB_NAME=openflare
go run main.go all
```

默认监听 `:3000`（由 `config.yaml` 的 `app.addr` 或 `APP_ADDR` 控制）。

## Docker 运行 Agent（推荐）

Docker 部署是 Agent 推荐的部署方式。Docker 部署时直接运行 Agent 镜像，该镜像基于 OpenResty 镜像制作，内置 Agent 控制器与 OpenResty 二进制。未显式配置 `node_ip` 时，Agent 会优先通过第三方 API 获取真实出口 IP，避免把 Docker 网桥地址登记为节点 IP。

挂载配置文件：

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -v openflare-agent-data:/data \
  -v ./agent.json:/etc/openflare/agent.json:ro \
  ghcr.io/rain-kl/openflare-agent:latest
```

使用环境变量：

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

## Agent 接入（脚本安装）

除了 Docker 部署外，也支持通过安装脚本将 Agent 部署在本地宿主机上。

使用 `discovery_token` 自动注册：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

使用节点专属 `agent_token`：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

安装脚本支持参数：

| 参数 | 说明 |
| --- | --- |
| `--server-url` | Server 地址，必填 |
| `--discovery-token` | 首次自动注册 Token，与 `--agent-token` 二选一 |
| `--agent-token` | 节点专属 Token，与 `--discovery-token` 二选一 |
| `--install-dir` | 安装目录，默认 `/opt/openflare-agent` |
| `--openresty-path` | OpenResty 二进制路径，未传时自动查找 `openresty` |
| `--repo` | 下载 Agent 的 GitHub 仓库，默认 `Rain-kl/OpenFlare` |
| `--no-service` | 不创建 systemd 服务 |

确认状态：

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

## 手动运行 Agent

源码运行：

```bash

export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

编译后二进制运行：

```bash

go build -o openflare-agent ./cmd/agent
export LOG_LEVEL='info'
./openflare-agent -config /path/to/agent.json
```

最小 `agent.json` 示例：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_path": "openresty",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

未配置 `openresty_path` 时，Agent 默认调用 `openresty`。

默认情况下，Agent 在 HTTP 心跳成功后会尝试升级为 WebSocket。升级成功时，Server 发布或激活配置会立即通知 Agent；如果 WebSocket 无法建立或意外断开，Agent 会自动退回 HTTP 心跳同步。

WAF 地域规则依赖 Agent 本地 `GeoLite2-Country.mmdb`。Agent 启动时会在 `data_dir/etc/openflare/GeoLite2-Country.mmdb` 初始化内置数据库，并按配置周期尝试更新；更新失败只记录警告，不影响配置同步与 OpenResty reload。

## 升级与卸载

Server：

* Root 用户可在管理端顶栏检查并升级正式版。
* 如需尝试 preview 版本，可手动检查对应发布。
* 也可通过上传 Server 二进制的方式执行确认升级。

Agent：

* Agent 默认只跟随正式版自动更新。
* Agent 自更新从 GitHub Release 拉取目标二进制，优先使用 Release API 的 `digest` 字段做 SHA-256 校验；仅当 digest 为空（历史 Release）时才回退读取同名 `.sha256` 侧车文件，校验通过后才替换本地可执行文件。
* 安装脚本可重复执行，用于重装或升级 Agent。
* preview 升级需要手动触发。

卸载 Agent：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

卸载脚本会停止 Agent、删除 systemd 服务和安装目录，不会删除本机 OpenResty。
