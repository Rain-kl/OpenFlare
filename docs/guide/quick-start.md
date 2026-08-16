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

| 项目 | 要求                                                               |
| --- |------------------------------------------------------------------|
| Docker / Docker Compose | 用于启动 Server 及其依赖的 PostgreSQL、Valkey；如采用 Docker Agent，也用于运行 Agent |
| OpenResty | 本地安装 Agent 时需要可执行 `openresty`，或在安装脚本中指定路径                        |
| 可访问端口 | Server 默认监听 `3000`，Agent 节点需要能访问 Server 地址                       |

---

## 1. 启动 Server

快速开始推荐采用 **PostgreSQL + Valkey** 标准部署方案。

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
      - openflare_uploads:/app/uploads
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
      REDIS_ADDR: "redis:6379"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openflare}
      POSTGRES_USER: ${DB_USERNAME:-openflare}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-replace-with-strong-password}
    volumes:
      - openflare_postgres_data:/var/lib/postgresql/data
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
      - openflare_redis_data:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5


volumes:
    openflare_uploads:
    openflare_postgres_data:
    openflare_redis_data:
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

如果忘记密码并且没有配置找回密码渠道，可以使用命令重置：

```bash
go run main.go reset-passwd --user admin
```

未指定 `--password` 时命令会自动生成随机密码并输出到终端；也可以使用 `--password` 显式指定新密码。

---

## 2. 准备 Agent Token

Agent 可以用两类凭证接入：

| 凭证 | 适用场景 |
| --- | --- |
| `discovery_token` | 首次自动注册节点，由 Server 换成节点专属 Token |
| `agent_token` | 已经在管理端创建或分配节点，直接使用节点专属 Token |

在管理端准备其中一种凭证后，进入下一步。

- **`discovery_token`** 获取菜单路径：「系统设置」->「OpenFlare」选项卡 ->「Discovery Token 与部署」中的 Discovery Token
- **`agent_token`** 获取菜单路径：在「节点管理」中创建节点后，点击进入节点详情页即可查看到对应的专属 Token。

---

## 3. 安装/运行 Agent

推荐使用 Docker 镜像部署 Agent；也可以通过安装脚本部署到本地宿主机。

### 方式 A：Docker 运行 Agent（推荐）

在代理节点上直接运行 Agent 镜像：

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

## 遇到问题时

按以下顺序处理：

1. 将 Server 与 Agent 升级到最新版本，确认问题是否仍然存在。
2. 重新发布并激活配置版本，等待节点应用。
3. 在节点详情页对目标节点执行「强制同步」，推动节点立即拉取最新配置。
4. 重建或重装 Agent（重新执行安装脚本）。
5. 上述步骤均无效时，携带 Server 日志与节点应用记录提交 [GitHub Issue](https://github.com/Rain-kl/OpenFlare/issues)。

更多排查思路见 [故障排查](./troubleshooting.md)。

---

## 进阶部署指引

当您完成快速开始并熟悉了 OpenFlare 的基本操作后，可以阅读以下进阶部署文档，将各组件投入到正式生产环境中：

* **Server 生产部署**：阅读 [启动 Server](../deployment/server.md) 了解如何从源码构建前端、配置系统环境变量及使用 Docker Compose 运行。
* **Agent 生产接入**：阅读 [部署 Agent](../deployment/agent.md) 了解基于 systemd 的服务管理、详细本地配置文件字段及故障排查。
* **内网穿透中继端部署**：阅读 [部署 Relay](../deployment/relay.md) 了解如何为穿透隧道配置公网中继节点（frps）。
* **内网穿透客户端部署**：阅读 [部署 OpenFlared](../deployment/openflared.md) 了解如何在内网服务器侧运行穿透守护客户端（frpc）。
* **生产部署拓扑参考**：阅读 [部署说明](../deployment/deployment.md) 了解生产高可用拓扑和整体网络规划。
* **系统升级与日常维护**：阅读 [升级与维护](../deployment/upgrade.md) 了解如何平滑升级 Server 和各代理节点 Agent。
