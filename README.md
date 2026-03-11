<p align="right">
  <strong>中文</strong> | <a href="./README.en.md">English</a>
</p>

[//]: # (<p align="center">)

[//]: # (  <a href="https://github.com/Rain-kl/ATSFlare"><img src="https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/atsf_server/web/public/logo.png" width="150" height="150" alt="ATSFlare logo"></a>)

[//]: # (</p>)

<div align="center">

# ATSFlare

_✨ control plane for reverse proxy management ✨_

</div>

<p align="center">
  <a href="https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/LICENSE">
    <img src="https://img.shields.io/github/license/Rain-kl/ATSFlare?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Rain-kl/ATSFlare/releases/latest">
    <img src="https://img.shields.io/github/v/release/Rain-kl/ATSFlare?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/Rain-kl/ATSFlare/releases/latest">
    <img src="https://img.shields.io/github/downloads/Rain-kl/ATSFlare/total?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://goreportcard.com/report/github.com/Rain-kl/ATSFlare">
    <img src="https://goreportcard.com/badge/github.com/Rain-kl/ATSFlare" alt="GoReportCard">
  </a>
</p>

[//]: # (<p align="center">)

[//]: # (  <a href="https://github.com/Rain-kl/ATSFlare/releases">Download</a>)

[//]: # (  ·)

[//]: # (  <a href="https://github.com/Rain-kl/ATSFlare/blob/main/README.en.md#deployment">Tutorial</a>)

[//]: # (  ·)

[//]: # (  <a href="https://github.com/Rain-kl/ATSFlare/issues">Feedback</a>)

[//]: # (</p>)



## 仓库结构

- `atsf_server`: Gin + GORM + SQLite 的控制中心，包含管理端 API、Agent API 和 Web 管理台
- `atsf_agent`: Go 单体 Agent，负责注册、心跳、同步配置、写入 Nginx 路由文件并 reload
- `docs`: 设计、开发规范、开发计划和部署联调文档


## 快速开始

### 1. 启动 Server

可直接使用 GHCR 镜像通过 Docker Compose 启动控制面：

```yaml
services:
  atsflare:
    image: ghcr.io/rain-kl/atsflare:latest
    container_name: atsflare
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-random-string
      SQLITE_PATH: /data/atsflare.db
      GIN_MODE: release
    volumes:
      - atsflare-data:/data

volumes:
  atsflare-data:
```

```bash
docker compose up -d
```

默认访问地址：`http://localhost:3000`。

- [docs/deployment.md](./docs/deployment.md)


### 2. 使用 Discovery Token 一键部署 Agent

适用于新节点首次接入，Agent 会使用全局 `discovery_token` 自动注册并换取节点专属 `agent_token`。

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

### 3. 使用 Agent Token 一键部署 Agent

适用于已经在管理端预创建节点、并拿到节点专属 `agent_token` 的场景。

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

说明：

* `--server-url` 替换为实际控制面地址，例如 `http://192.168.1.10:3000`
* Linux 默认安装到 `/opt/atsflare-agent`，并创建 `atsflare-agent` systemd 服务
* 重复执行相同命令可用于升级 Agent 到最新 Release
* Root 用户可在管理端顶栏点击“版本”检查最新 Release，并在 Release 二进制部署场景下直接触发 Server 自升级


## 部署说明

当前仓库的交付形式：

* Server 二进制发布到 GitHub Releases
* Server Docker 镜像发布到 GitHub Container Registry：`ghcr.io/rain-kl/atsflare`
* Agent 二进制发布到 GitHub Releases

详细文档：

* [docs/deployment.md](./docs/deployment.md)
* [docs/design.md](./docs/design.md)


## 贡献

参与开发请先阅读：

1. [docs/design.md](./docs/design.md)
2. [docs/development-guidelines.md](./docs/development-guidelines.md)
3. [docs/development-plan.md](./docs/development-plan.md)

前端开发补充：

* 新版管理端位于 `atsf_server/web`
* 前端包管理器统一使用 `pnpm`
* 构建命令为 `pnpm build`，产物输出到 `atsf_server/web/build`

