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
| Docker / Docker Compose | 用于启动 Server 和 PostgreSQL；如果采用 Docker Agent 镜像，也用于运行 Agent |
| OpenResty | 本地安装 Agent 时需要可执行 `openresty`，或在安装脚本中指定路径 |
| 可访问端口 | Server 默认监听 `3000`，Agent 节点需要能访问 Server 地址 |
| 浏览器 | 用于访问管理端 |

- **Docker**：`20.10.0+`
- **Docker Compose**：`2.0.0+`

## 1. 启动 Server

在空目录中创建 `docker-compose.yml`：

```yaml
services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: openflare
      POSTGRES_USER: openflare
      POSTGRES_PASSWORD: replace-with-strong-password
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openflare -d openflare"]
      interval: 10s
      timeout: 5s
      retries: 5

  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-a-long-random-string
      DSN: postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info
    volumes:
      - openflare-data:/data

volumes:
  postgres-data:
  openflare-data:
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

看到 `server listening` 且 `openflare` 容器状态为 running 后，访问：

```text
http://localhost:3000
```

默认账号：

| 用户名 | 密码 |
| --- | --- |
| `root` | `123456` |

首次登录后请立即修改默认密码。

## 2. 准备 Agent Token

Agent 可以用两类凭证接入：

| 凭证 | 适用场景 |
| --- | --- |
| `discovery_token` | 首次自动注册节点，由 Server 换成节点专属 Token |
| `agent_token` | 已经在管理端创建或分配节点，直接使用节点专属 Token |

在管理端准备其中一种凭证后，进入下一步。

- **`discovery_token`** 获取菜单路径：「系统设置」->「自动注册」
- **`agent_token`** 获取菜单路径：「节点管理」->「新增节点」

## 3. 安装/运行 Agent

Agent 部署方式推荐使用 Docker 部署（即直接运行内置 OpenResty 的 Agent 镜像）；亦支持通过安装脚本将 Agent 部署在本地宿主机上。

### 方式 A：Docker 运行 Agent（推荐）

在代理节点上直接运行 Agent 镜像：

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -v openflare-agent-data:/data \
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

## 4. 发布第一份配置

在管理端完成以下操作：

1. 新增网站配置，填写网站名称、域名和源站地址。
2. 确认网站配置处于启用状态。
3. 发布前查看预览或变更摘要。
4. 发布并激活新版本。
5. 等待 Agent 在后续 heartbeat 中发现版本并应用。

版本号格式为 `YYYYMMDD-NNN`。历史版本不可变，回滚通过重新激活旧版本完成。

## 5. 验证是否成功

在管理端确认：

| 位置 | 期望结果 |
| --- | --- |
| 节点列表 | Agent 节点在线 |
| 节点详情 | 当前版本与激活版本一致 |
| 应用记录 | 最近一次应用成功 |
| 版本页面 | 新版本处于激活状态 |

在 Agent 节点确认：

```bash
journalctl -u openflare-agent -n 100 --no-pager
```

## 常见失败原因

| 现象 | 排查方向 |
| --- | --- |
| 浏览器打不开管理端 | 确认 `docker compose ps` 中 Server 正在运行，宿主机 `3000` 端口没有被占用 |
| 登录后数据无法保存 | 检查 PostgreSQL 容器健康状态，以及 `DSN` 中的用户名、密码、库名是否一致 |
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

