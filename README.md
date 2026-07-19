<div align="center">

# OpenFlare

**[📖 中文](./README.md) | [English](./README.en.md)**

OpenFlare 是开源 CDN 编排与边缘安全平台。它支持反向代理、集中式配置同步、内网穿透（Tunnels）、动态 WAF 防护以及防 CC 挑战。

</div>

<p align="center">
  <a href="https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/LICENSE">
    <img src="https://img.shields.io/github/license/Rain-kl/OpenFlare?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/releases/latest">
    <img src="https://img.shields.io/github/v/release/Rain-kl/OpenFlare?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/pkgs/container/openflare">
    <img src="https://img.shields.io/badge/GHCR-ghcr.io%2Frain--kl%2Fopenflare-brightgreen" alt="ghcr">
  </a>
</p>

> [!WARNING]
> 使用 `admin` 用户初次登录系统后，务必修改默认密码 `12345678`。
>
> BETA 版本为开发测试阶段的临时产物，可能存在未知问题，请勿在生产环境使用。

## 文档

**https://open-flare.pages.dev**

常用入口：

* [快速开始](https://open-flare.pages.dev/guide/quick-start)
* [部署说明](https://open-flare.pages.dev/deployment/deployment)
* [配置项参考](https://open-flare.pages.dev/reference/configuration)
* [系统设计](https://open-flare.pages.dev/design/)

## 核心能力

* **反代配置管理**：以网站规则为聚合边界，支持多域名绑定与多上游负载均衡，统一管理所有 OpenResty 节点的反代配置。
* **安全内网穿透（Tunnels）**：开源版的 Cloudflare Tunnels。无须公网 IP 或暴露入向端口，通过 Relay 中继节点与 OpenFlared 客户端安全反向穿透内网 Web 服务至公网。
* **边缘 WAF 安全防护**：提供全局与自定义规则组，支持手动/自动/订阅型 IP 组、MaxMind GeoIP 国家级地域准入、IP 组成员 Checksum 差分同步（无需 Nginx 重载）以及自定义拦截响应。
* **防 CC 与人机挑战（PoW）**：内置高性能客户端密码学 Proof of Work 挑战（类似 Turnstile），在网关边缘秒级拦截并阻断僵尸网络与爬虫。
* **Pages 静态托管**：支持上传或从受限 Remote URL、公开 GitHub Release asset 同步预构建产物；GitHub latest 可定时检查并可选自动发布。所有来源统一生成不可变部署，由边缘 Agent 拉取并通过 OpenResty 本地提供服务，支持回滚、SPA Fallback 与 API 反向代理。
* **TLS 证书自动化**：支持证书动态上传、多域名证书自动匹配绑定，以及通过 ACME 协议向 Let's Encrypt 自动申请与续期证书。
* **Uptime Kuma 监控同步**：与 Uptime Kuma 集成，自动差分同步监控站点列表，实时感知节点存活与服务可用状态。
* **SSO 单点登录**：支持 GitHub OAuth 与标准 OIDC 协议，无缝接入企业身份提供商实现统一登录。
* **统一观测**：聚合节点请求指标、实时访问日志明细、宿主机与 Nginx 资源快照、健康事件以及网络波动补传缓冲。

## 界面预览

### 仪表盘总览

![OpenFlare dashboard overview](./docs/assets/readme/dashboard-overview.png)

### 节点详情

![OpenFlare node detail](./docs/assets/readme/node-detail.png)

### 配置新增

![OpenFlare version release](./docs/assets/readme/proxy-route-detail.png)

## 快速开始

### 1. 启动 Server

使用 docker-compose

```bash
# 下载环境变量模板并创建 .env 文件
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env

# ClickHouse 服务端：curl performance.xml 到 ./config/clickhouse，并以单文件方式挂载到 config.d
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
```

```yaml
services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    env_file: .env
    environment:
      TZ: ${TZ:-Asia/Shanghai}
    ports:
      - "3000:3000"
    volumes:
      - openflare_uploads:/app/uploads
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
      start_period: 5s

  clickhouse:
    image: clickhouse/clickhouse-server:25.3-alpine
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_NAME:-openflare}
      CLICKHOUSE_USER: ${CLICKHOUSE_USERNAME:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      TZ: ${TZ:-Asia/Shanghai}
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    volumes:
      - openflare_clickhouse_data:/var/lib/clickhouse
      - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
    healthcheck:
      test: ["CMD", "clickhouse-client", "--user", "${CLICKHOUSE_USERNAME:-default}", "--password", "${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s

volumes:
    openflare_uploads:
    openflare_postgres_data:
    openflare_redis_data:
    openflare_clickhouse_data:
```

详细部署说明见 [部署文档](https://open-flare.pages.dev/deployment/deployment)。

访问地址：`http://localhost:3000`

默认账号：

* 用户名：`admin`
* 密码：`12345678`

### 2. 安装 Agent

安装 Agent 前请先在节点上安装 OpenResty，或改用内置 OpenResty 的 Agent Docker 镜像。

你可以在控制面板的节点管理->详情->节点信息->节点标识与部署复制安装命令，或直接使用下面的脚本：

#### Docker 部署

Docker 部署可直接运行 Agent 镜像：

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

## 开源协议

本项目采用 [Apache License 2.0](./LICENSE) 开源。

## Star History

<a href="https://www.star-history.com/?repos=Rain-kl%2FOpenFlare&type=date&legend=bottom-right">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&legend=top-left" />
 </picture>
</a>
