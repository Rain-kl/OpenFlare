# 部署说明

你会学到：OpenFlare 的推荐部署方式、Server 与 Agent 的运行要求、源码启动方式、联调步骤、升级与卸载入口。

生产环境建议使用 PostgreSQL 作为 Server 数据库，并通过 `config.yaml` 或环境变量配置 `APP_SESSION_SECRET` 等参数。完整 Docker Compose 部署需要 Redis；ClickHouse 可选，用于海量访问日志与观测时序（见仓库根目录 `docker-compose.yaml`）。Agent 支持 Docker 部署与本地安装脚本两种方式，Docker 镜像已内置 Pingora 二进制。日志库判定与切换见 [日志存储解耦](../design/logstore.md)。

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
Pingora binary
  |
  v
Origin service
```

### 内网穿透流量路径

```text
Browser
  |
  v
Pingora (Agent, WAF/HTTPS 终结)      <-- TunnelRelay 节点
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
| **Agent 数据面** | 1 核 CPU / 512 MB 内存 / 2 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 10 GB+ 磁盘 | 根据 Pingora 的并发代理连接量与 WAF 拦截处理扩容 |
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
go run ./cmd/server all
```

默认监听 `:3000`（由 `config.yaml` 的 `app.addr` 或 `APP_ADDR` 控制）。

## 部署边缘节点

按 [Pingora Edge 部署说明](./agent.md) 安装。发布站点快照并确认节点在线后，可在「性能策略」批量分配节点。
