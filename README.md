<p align="right">
  <strong>中文</strong>
</p>

<div align="center">

# GinNextTemplate

Gin + Next.js 的后台管理模板工程，保留用户、邮箱、文件上传、安全和服务端升级等基础能力，适合作为新项目的起点。
</div>

## 项目定位

`GinNextTemplate` 是从历史 OpenFlare 工程裁剪而来的模板工程。当前主线不是继续扩展 OpenFlare，而是沉淀一套可复用、可维护、可继续二次开发的全栈基础底座。

当前长期保留模块：

* 用户与认证
* 邮箱能力
* 文件上传
* 安全能力
* 服务端版本升级

当前正在移除的历史模块：

* Agent
* 节点与心跳同步
* 配置版本分发
* OpenResty 代理能力
* 域名与证书分发
* 观测分析与看板

## 当前工程目标

* 统一项目名称为 `GinNextTemplate`
* 服务端严格遵循 MVC 开发
* 目录结构逐步迁移到标准 Go 布局：`cmd/`、`internal/`、`pkg/`
* 接口与界面同步裁剪，避免残留历史入口

## 快速开始

### 1. 启动 Server

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

```bash
cd openflare_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./ginnexttemplate.db'
export LOG_LEVEL='info'
go run .
```

访问地址：`http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

### 2. 本地开发

Server：

```bash
cd openflare_server
go run .
```

Frontend：

```bash
cd openflare_server/web
pnpm install
pnpm dev
```

## 仓库结构

当前仓库仍在迁移过程中，现有目录主要包括：

* `openflare_server`：当前服务端代码
* `openflare_server/web`：当前管理端前端代码
* `docs`：设计、规范、计划、部署和配置文档

目标结构会逐步迁移到：

* `cmd/`
* `internal/handler`
* `internal/service`
* `internal/repository`
* `internal/model`
* `internal/dto`
* `internal/middleware`
* `internal/pkg`
* `pkg/`

## 常用命令

Server 测试：

```bash
cd openflare_server
go test ./...
```

Frontend 构建：

```bash
cd openflare_server/web
pnpm build
```

## 文档

建议按以下顺序阅读：

1. [docs/template-refactor-plan.md](./docs/template-refactor-plan.md)
2. [docs/design.md](./docs/design.md)
3. [docs/development-guidelines.md](./docs/development-guidelines.md)
4. [docs/development-plan.md](./docs/development-plan.md)
5. [docs/deployment.md](./docs/deployment.md)
6. [docs/app-config.md](./docs/app-config.md)

## 开源协议

本项目采用 [Apache License 2.0](./LICENSE) 开源。
