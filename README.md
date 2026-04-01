<p align="right">
  <strong>中文</strong> | <a href="./README.en.md">English</a>
</p>

<div align="center">

[//]: # (  <img src="./openflare_server/web/public/logo.png" width="120" height="120" alt="OpenFlare logo">)

# OpenFlare

轻量、自托管的 OpenResty 控制面，用于管理反向代理规则、配置发布、节点同步、TLS 证书与基础可观测能力。

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

> [!NOTE]
> 当前项目处于快速迭代期，设计与实现均不稳定，请确保使用最新版本并关注更新日志。

> [!WARNING]
> 使用 root 用户初次登录系统后，务必修改默认密码 `123456`，并且确保关闭新用户注册功能。

## 为什么存在

OpenFlare 解决的是一类朴素但高频的运维问题：

* 在一个管理端里维护域名到源站的反向代理规则
* 生成完整 OpenResty 配置并以不可变版本发布
* 让节点侧 Agent 自动拉取、校验、reload 与失败回滚
* 统一托管证书、域名、节点凭证与版本状态
* 提供足够实用的总览、节点详情与访问分析能力

## 核心能力

* 配置版本化：支持预览、发布、激活、历史回滚
* Agent 自动应用：周期性同步、落盘、`openresty -t`、`openresty -s reload`、失败自动回滚
* OpenResty 托管：统一管理主配置模板、性能参数、缓存参数与受管路由
* TLS 与域名管理：支持证书托管、域名资产维护、精确匹配与通配符匹配
* 访问与节点观测：支持请求窗口聚合、状态码分布、来源分布、节点资源与健康事件展示

## 系统架构

```text
OpenFlare Server (Gin + GORM + SQLite/PostgreSQL + Web UI)
        |
        | HTTP API / Config Pull
        v
OpenFlare Agent (register / heartbeat / sync / apply / update)
        |
        v
Local OpenResty or Docker OpenResty
        |
        v
Origin
```

职责划分：

* `openflare_server`：管理端 UI、管理 API、Agent API、配置渲染、版本发布与状态存储
* `openflare_agent`：节点注册、心跳、同步、本地写入、校验、reload、回滚、自更新
* `openflare_server/web`：新版管理端前端，静态导出后由 Go Server 托管

## 界面预览

### 仪表盘总览

![OpenFlare dashboard overview](./docs/assets/readme/dashboard-overview.png)

### 节点详情

![OpenFlare node detail](./docs/assets/readme/node-detail.png)

### 配置新增

![OpenFlare version release](./docs/assets/readme/version-release.png)

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

### 2. 接入 Agent

**注意:** 安装agent前需确保存已经安装了Docker, 虽然支持裸Openresty,但未得到充分验证,可能存在未知问题.

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

安装脚本默认写入 `/opt/openflare-agent`，创建 `openflare-agent.service`，并可重复执行以重装或升级 Agent。

如需彻底卸载 Agent 并清空本地数据，可执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

卸载脚本会先停止并移除 `openflare-agent.service`、删除整个 `/opt/openflare-agent` 目录，然后根据卸载前保存的 `agent.json` 判断 OpenResty 安装方式：

* Docker 模式：删除对应 OpenResty 容器，并尝试移除镜像
* 本机 `openresty_path` 模式：不改动本机 OpenResty，只提示用户手动卸载

### 3. 发布第一份配置

1. 登录管理端并新增反代规则
2. 在发布前查看预览或变更摘要
3. 激活新版本
4. 等待 Agent 在后续 heartbeat 中拉取并应用配置

版本号格式固定为 `YYYYMMDD-NNN`，历史版本不可变，回滚通过重新激活旧版本完成。

## 仓库结构

* `openflare_server`：Gin + GORM + SQLite/PostgreSQL 单体控制面
* `openflare_server/web`：Next.js 15 App Router 管理端前端
* `openflare_agent`：Go 单体 Agent
* `scripts`：安装脚本与辅助脚本
* `docs`：设计、规范、部署与配置文档

## 本地开发

### Server

```bash
cd openflare_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./openflare.db'
# 可选：设置 DSN 或 SQL_DSN 后切换到 PostgreSQL。
# 如果 PostgreSQL 为空且 ./openflare.db 存在，启动时会自动迁移 SQLite 数据。
# export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
go run .
```

### Frontend

```bash
cd openflare_server/web
pnpm install
pnpm dev
```

### Agent

```bash
cd openflare_agent
go run ./cmd/agent -config /path/to/agent.json
```

## 管理端与接口

管理端当前覆盖：

* 反代规则
* 配置版本
* 节点管理
* 应用记录
* TLS 证书
* 域名管理
* 用户管理
* 设置
* 版本更新

登录管理端后，可访问 Swagger UI：`/swagger/index.html`

## 开源协议

本项目采用 [Apache License 2.0](./LICENSE) 开源。
