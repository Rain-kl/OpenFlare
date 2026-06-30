# 快速开始

你会学到：如何用 Docker Compose 启动 OpenFlare Server、完成首次登录、接入第一个 Agent，并验证一份配置是否已经发布到节点。

OpenFlare 的最小运行单元包含：

| 组件 | 职责 |
| --- | --- |
| Server | 管理端 UI、管理 API、Agent API、配置渲染、版本发布与状态存储 |
| Agent | 运行在代理节点上，拉取配置、写入 OpenResty、执行校验与 reload |
| OpenResty | 实际接收流量并反向代理到源站 |

Agent 统一通过 OpenResty 二进制控制运行时。本地部署需要节点上已有 `openresty` 可执行文件；Docker 部署可直接运行内置 OpenResty 的 Agent 镜像。

## 环境要求

| 项目 | 要求 |
| --- | --- |
| Docker / Docker Compose | 用于启动 Server 及其依赖的 PostgreSQL、Redis 和 ClickHouse 容器；如采用 Docker Agent，也用于运行 Agent |
| OpenResty | 本地安装 Agent 时需要可执行 `openresty`，或在安装脚本中指定路径 |
| 可访问端口 | Server 默认监听 `3000`，Agent 节点需要能访问 Server 地址 |
| 浏览器 | 用于访问管理端 |

- **Docker**：`20.10.0+`
- **Docker Compose**：`2.0.0+`

---

## 1. 启动 Server

为了保证异步任务队列（Asynq 框架）及可观测流量看板功能完整运行，快速开始推荐采用 **PostgreSQL + Redis + ClickHouse** 经典单机版编排。

在空目录中创建 `docker-compose.yaml`：

```yaml
version: '3.8'

services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    container_name: openflare-server
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - ./uploads:/app/uploads
    environment:
      TZ: Asia/Shanghai
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # 生产环境请替换为长随机字符串
      DB_ENABLED: "true"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USERNAME: "${DB_USERNAME:-openflare}"
      DB_PASSWORD: "${DB_PASSWORD:-replace-with-strong-password}"
      DB_NAME: "${DB_NAME:-openflare}"
      REDIS_ENABLED: "true"
      REDIS_ADDRS: "redis:6379"
      CLICKHOUSE_ENABLED: "true"
      CLICKHOUSE_HOSTS: "clickhouse:9000"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      clickhouse:
        condition: service_healthy

  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openflare}
      POSTGRES_USER: ${DB_USERNAME:-openflare}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-replace-with-strong-password}
    volumes:
      - ./data/postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USERNAME:-openflare} -d ${DB_NAME:-openflare}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: valkey/valkey:8.0-alpine
    restart: unless-stopped
    command: ["valkey-server", "--appendonly", "yes"]
    volumes:
      - ./data/valkey:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  clickhouse:
    image: clickhouse/clickhouse-server:25.3-alpine
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: openflare
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      TZ: Asia/Shanghai
    volumes:
      - ./data/clickhouse_data:/var/lib/clickhouse
    healthcheck:
      test: ["CMD", "clickhouse-client", "--user", "${CLICKHOUSE_USERNAME:-default}", "--password", "${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s
```

启动服务：

```bash
docker compose up -d
```

确认容器已经运行：

```bash
docker compose ps
docker compose logs -f openflare
```

看到 `server listening` 且 `openflare-server` 容器状态为 running 后，使用浏览器打开：

```text
http://localhost:3000
```

默认账号：

| 用户名 | 密码 |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> 为了你的系统安全，首次登录后请立即修改默认密码。

---

## 2. 准备 Agent Token

Agent 可以用两类凭证接入：

| 凭证 | 适用场景 |
| --- | --- |
| `discovery_token` | 首次自动注册节点，由 Server 换成节点专属 Token |
| `agent_token` | 已经在管理端创建或分配节点，直接使用节点专属 Token |

在管理端准备其中一种凭证后，进入下一步。

- **`discovery_token`** 获取菜单路径：「系统设置」 (Settings) -> 「OpenFlare」选项卡 -> 「自动注册」凭证
- **`agent_token`** 获取菜单路径：在「节点管理」中创建节点后，点击进入节点详情页即可查看到对应的专属 Token。

---

## 3. 安装/运行 Agent

Agent 部署方式推荐使用 Docker 部署（即直接运行内置 OpenResty 的 Agent 镜像）；亦支持通过安装脚本将 Agent 部署在本地宿主机上。

### 方式 A：Docker 运行 Agent（推荐）

在代理节点上直接运行 Agent 镜像：

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

### 方式 B：执行安装脚本（本地部署）

在代理节点上执行安装脚本。

使用 `discovery_token`：

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

脚本默认会：

| 项目 | 默认值 |
| --- | --- |
| 安装目录 | `/opt/openflare-agent` |
| 配置文件 | `/opt/openflare-agent/agent.json` |
| systemd 服务 | `openflare-agent.service` |
| OpenResty 路径 | 未指定时自动查找 `openresty` |

确认 Agent 服务状态：

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

如果没有 systemd，脚本会输出手动启动命令。

---

## 4. 后续步骤

完成控制面板启动和 Agent 节点接入后，你已经成功搭建好了 OpenFlare 网关的基础运行环境。接下来你可以按顺序继续阅读以下两份指南，开始部署你的第一个反代站点：

1. **发布第一个网站**：
   * 请参阅 [发布第一份配置](./first-site.md)。它将引导你以最简单的方式（使用纯 HTTP）发布你的第一条代理规则，并验证节点落地状态。
2. **完整配置反向代理（HTTPS 与源站管理）**：
   * 请参阅 [新建反代配置](./proxy-config.md)。它将指导你从证书导入与申请开始，配置域名 HTTPS 证书绑定、源站管理并预览发布。

---

## 常见失败原因

| 现象 | 排查方向 |
| --- | --- |
| 浏览器打不开管理端 | 确认 `docker compose ps` 中 Server 正在运行，宿主机 `3000` 端口没有被占用 |
| 登录后数据无法保存/提示报错 | 检查 PostgreSQL 容器健康状态，以及 `DB_PASSWORD` / 密码等连接参数是否一致 |
| Agent 无法注册 | 确认 Agent 节点能访问 `--server-url`，并检查 Token 是否填错或已失效 |
| Agent 在线但没有应用配置 | 确认网站配置已启用，并且已经发布并激活版本 |
| OpenResty 应用失败 | 查看节点应用记录和 `journalctl -u openflare-agent`，重点检查域名、证书、上游地址和端口占用 |

更多排查路径见 [故障排查](./troubleshooting.md)。

---

## 进阶部署指引

当您完成快速开始并熟悉了 OpenFlare 的基本操作后，可以阅读以下进阶部署文档，将各组件投入到正式生产环境中：

* **Server 生产部署**：阅读 [启动 Server](../deployment/server.md) 了解如何从源码构建前端、配置系统环境变量及使用 Docker Compose 运行。
* **Agent 生产接入**：阅读 [部署 Agent](../deployment/agent.md) 了解基于 systemd 的服务管理、详细本地配置文件字段及故障排查。
* **内网穿透中继端部署**：阅读 [部署 Relay](../deployment/relay.md) 了解如何为穿透隧道配置公网中继节点（frps）。
* **内网穿透客户端部署**：阅读 [部署 OpenFlared](../deployment/openflared.md) 了解如何在内网服务器侧运行穿透守护客户端（frpc）。
* **生产部署拓扑参考**：阅读 [部署说明](../deployment/deployment.md) 了解生产高可用拓扑和整体网络规划。
* **系统升级与日常维护**：阅读 [升级与维护](../deployment/upgrade.md) 了解如何平滑升级 Server 和各代理节点 Agent。
