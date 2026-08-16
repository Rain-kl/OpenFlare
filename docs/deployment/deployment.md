# 部署说明

你会学到：OpenFlare 的推荐部署方式、Server 与 Agent 的运行要求、源码启动方式、联调步骤、升级与卸载入口。

生产环境建议使用 PostgreSQL 作为 Server 数据库，并通过 `config.yaml` 或环境变量配置 `APP_SESSION_SECRET` 等参数。完整 Docker Compose 部署需要 Redis；ClickHouse 可选，用于海量访问日志与观测时序（见仓库根目录 `docker-compose.yaml`）。Agent 支持 Docker 部署与本地安装脚本两种方式，Docker 镜像已内置 OpenResty 二进制。日志库判定与切换见 [日志存储解耦](../design/logstore.md)。

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

### 硬件配置推荐

| 组件 | 参考配置（入门）                    | 参考配置（生产） | 说明 |
| --- |-------------------------------| --- | --- |
| **Server 控制面** | 1 核 CPU / 2 GB 内存 / 20 GB 磁盘  | 2 核 CPU / 4 GB 内存 / 50 GB+ 磁盘 | 磁盘用量需根据访问日志留存时长与并发流量合理扩容 |
| **Agent 数据面** | 1 核 CPU / 512 MB 内存 / 2 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 10 GB+ 磁盘 | 根据 OpenResty 的并发代理连接量与 WAF 拦截处理扩容 |
| **Relay 中继节点**| 1 核 CPU / 1 GB 内存 / 5 GB 磁盘   | 2 核 CPU / 2 GB 内存 / 20 GB 磁盘 | frps 传输中继吞吐量主要受带宽与 CPU 吞吐能力限制 |
| **OpenFlared 客户端**| 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 1 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 独立运行于内网，自身资源占用极小，保障网络吞吐即可 |

## Docker Compose 部署 Server

仓库根目录已提供完整 `docker-compose.yaml`（含 PostgreSQL、Redis、ClickHouse、Jaeger）。

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env，至少修改 APP_SESSION_SECRET 与数据库密码
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

首次访问 `http://localhost:3000`，默认账号为 `admin` / `12345678`。登录后请立即修改默认密码。

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

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -v openflare-agent-pages:/data/var/lib/openflare/pages \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

命名卷 `openflare-agent-pages` 持久化 Pages 部署目录，重建容器时无需重新拉取静态站点包。

## Agent 接入（脚本安装）

除了 Docker 部署外，也支持通过安装脚本将 Agent 部署在本地宿主机上。安装脚本会自动在本地 Linux 系统中注册低权限的 `openflare` 服务账号，并将 systemd 服务配置为以该用户身份运行，利用 Linux Capabilities 安全地监听 80/443 特权端口。

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
