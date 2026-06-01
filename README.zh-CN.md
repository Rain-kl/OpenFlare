<div align="center">

# OpenFlare

轻量、自托管的 OpenResty 控制面，用于管理反向代理规则、配置发布、节点同步、TLS 证书与基础可观测能力。

</div>

<p align="center
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
> 使用 `root` 用户初次登录系统后，务必修改默认密码 `123456`。
> 
> BETA 版本为开发测试阶段的临时产物，可能存在未知问题，请勿在生产环境使用。

## 文档

**https://open-flare.pages.dev**

常用入口：

* [快速开始](https://open-flare.pages.dev/guide/quick-start)
* [部署说明](https://open-flare.pages.dev/guide/deployment)
* [配置项参考](https://open-flare.pages.dev/reference/configuration)
* [系统设计](https://open-flare.pages.dev/design/)

## 核心能力

* 反向代理网站配置与多域名绑定
* 配置预览、发布、激活与历史回滚
* Agent 自动注册、心跳、同步、校验、reload 与失败回滚
* OpenResty 主配置、性能参数、缓存参数与 Lua 资源托管
* WAF 全局/自定义规则组，支持 IP/IP 段与国家级地域黑白名单
* TLS 证书、域名资产、节点凭证与版本状态管理
* 请求聚合、访问分析、资源快照、健康事件与节点详情

## 快速开始

### 1. 启动 Server

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
      SESSION_SECRET: replace-with-random-string
      DSN: postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info

volumes:
  postgres-data:
```

```bash
docker compose up -d
```

访问地址：`http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

### 2. 安装 Agent

安装 Agent 前请先在节点上安装 OpenResty，或改用内置 OpenResty 的 Agent Docker 镜像。

你可以在控制面板的节点管理->详情->节点信息->节点标识与部署复制安装命令，或直接使用下面的脚本：

#### Docker 部署

Docker 部署可直接运行 Agent 镜像：

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

#### 本地部署

使用 `discovery_token` 接入：

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

安装脚本默认写入 `/opt/openflare-agent`，创建 `openflare-agent.service`，自动查找 `openresty`，并可重复执行以重装或升级 Agent。

### 3. 卸载 Agent

如需彻底卸载 Agent 并清空本地数据，可执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

卸载脚本会先停止并移除 `openflare-agent.service`、删除整个 `/opt/openflare-agent` 目录，不会删除本机 OpenResty。

### 4. 发布第一份配置

1. 登录管理端并新增反代规则
2. 在发布前查看预览或变更摘要
3. 激活新版本
4. Agent 通过 WebSocket 通知或后续 heartbeat 拉取并应用配置

版本号格式固定为 `YYYYMMDD-NNN`，历史版本不可变，回滚通过重新激活旧版本完成。


## 界面预览

### 仪表盘总览

![OpenFlare dashboard overview](./docs/assets/readme/dashboard-overview.png)

### 节点详情

![OpenFlare node detail](./docs/assets/readme/node-detail.png)

### 配置新增

![OpenFlare version release](./docs/assets/readme/proxy-route-detail.png)

## 管理端与接口

管理端当前覆盖：

* 反代规则
* 配置版本
* 节点管理
* 应用记录
* TLS 证书
* 域名管理
* WAF 规则组
* 用户管理
* 设置
* 版本更新
* POW 规则

登录管理端后，可访问 Swagger UI：`/swagger/index.html`

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
