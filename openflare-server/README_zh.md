# wavelet

🚀 现代化、生产就绪的全栈应用脚手架

[English](./README.md)

[![License: Apache2.0](https://img.shields.io/badge/License-Apache2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-blue.svg)](https://reactjs.org/)

## 📖 项目简介

**wavelet** 是一个通用型、生产就绪的现代全栈脚手架，后端采用 **Go（Gin + GORM）**，前端采用 **Next.js（App Router + Shadcn UI）**。项目开箱即用，内置构建现代 SaaS、内部工具或开发者平台所需的核心基础设施。

项目设计理念是 **框架优先、业务中立**：您可以在沿用经过实战检验的底层基础设施的同时，自由接入自己的业务逻辑。

### ✨ 主要特性

- 🔐 **多认证方式** — 本地账号密码登录/注册 + 可插拔 OIDC/OAuth2 认证源（支持同时配置多个认证源）
- 🗝️ **个人访问令牌** — API Key 管理，支持程序化接口访问；兼容 `Authorization: Bearer` 和 `X-Access-Token` 请求头
- 👤 **用户管理** — 管理后台提供用户列表、搜索筛选、启用/禁用账号等功能
- ⚙️ **动态系统配置** — KV 系统配置管理，支持实时变更，可通过管理后台界面直接操作
- 📋 **异步任务队列** — 基于 [Asynq](https://github.com/hibiken/asynq)（Redis 驱动）的后台任务处理系统，含任务调度面板
- 📁 **S3 文件存储** — 通过 S3 兼容 API 统一处理文件上传/下载，支持本地磁盘缓存
- 📊 **可观测性** — 结构化日志（Zap）+ 分布式链路追踪（OpenTelemetry）
- 🎨 **现代化 UI** — 基于 Tailwind CSS 4 和 Shadcn UI 构建的响应式、支持深色模式的设计系统
- 📖 **内置文档中心** — 集成文档门户，包含使用指南、接口文档、隐私政策和服务条款

## 🏗️ 架构概览

```
┌─────────────────┐    ┌─────────────────────────────┐    ┌─────────────────┐
│   前端           │    │            后端              │    │   数据库         │
│   (Next.js)     │◄──►│           (Go)               │◄──►│  (PostgreSQL)   │
│                 │    │                              │    │                 │
│ • React 19      │    │ • Gin HTTP 框架              │    │ • PostgreSQL    │
│ • TypeScript    │    │ • GORM ORM                   │    │ • Redis 缓存    │
│ • Tailwind 4    │    │ • 多认证源适配               │    │                 │
│ • Shadcn UI     │    │ • AccessToken 中间件         │    │                 │
│                 │    │ • Asynq 任务队列             │    │                 │
│                 │    │ • OpenTelemetry 链路追踪     │    │                 │
│                 │    │ • Swagger 接口文档           │    │                 │
└─────────────────┘    └─────────────────────────────┘    └─────────────────┘
                                      │
                           ┌──────────┴──────────┐
                           │   多进程 CLI 入口    │
                           │  (Cobra + Viper)     │
                           │ • api      (HTTP)    │
                           │ • worker   (队列)    │
                           │ • scheduler(定时)    │
                           └─────────────────────┘
```

## 🛠️ 技术栈

### 后端
- **[Go 1.25+](https://go.dev/doc)** — 主语言
- **[Gin](https://github.com/gin-gonic/gin)** — HTTP Web 框架
- **[GORM](https://github.com/go-gorm/gorm)** — ORM，支持 PostgreSQL 和 ClickHouse
- **[Redis](https://github.com/redis/redis)** — 缓存、Session 存储、任务队列后端
- **[Asynq](https://github.com/hibiken/asynq)** — 分布式任务队列（Redis 驱动）
- **[Cobra + Viper](https://github.com/spf13/cobra)** — CLI 入口 + 配置管理
- **[OpenTelemetry](https://opentelemetry.io)** — 分布式链路追踪与可观测性
- **[Zap](https://github.com/uber-go/zap)** — 结构化高性能日志
- **[Swagger (Swaggo)](https://github.com/swaggo/swag)** — 自动生成 API 文档
- **[AWS SDK v2](https://github.com/aws/aws-sdk-go-v2)** — S3 兼容文件存储
- **[Snowflake](https://github.com/bwmarrin/snowflake)** — 分布式 ID 生成

### 前端
- **[Next.js 16](https://github.com/vercel/next.js)** — React 框架（App Router）
- **[React 19](https://github.com/facebook/react)** — UI 库
- **[TypeScript](https://github.com/microsoft/TypeScript)** — 类型安全
- **[Tailwind CSS 4](https://github.com/tailwindlabs/tailwindcss)** — 原子化 CSS 框架
- **[Shadcn UI](https://github.com/shadcn-ui/ui)** — 可访问、可组合的组件库
- **[Lucide Icons](https://github.com/lucide-icons/lucide)** — 图标库

## 📋 环境要求

- **Go** >= 1.25
- **Node.js** >= 18.0
- **PostgreSQL** >= 14
- **Redis** >= 6.0
- **pnpm** >= 8.0（推荐）

## 🚀 快速开始

### 1. 克隆仓库

```bash
git clone https://github.com/Rain-kl/Wavelet.git refreshing
cd refreshing
```

### 2. 配置环境

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml`，配置数据库和 Redis。OIDC 认证源统一在管理后台的系统设置页面运行时配置。

### 3. 初始化数据库

```bash
# 启动本地依赖服务（PostgreSQL + Redis）
docker compose up -d

# 可选：同时启动 ClickHouse
docker compose --profile clickhouse up -d

# 如果使用外部 PostgreSQL，而不是 Docker 内置服务，则手动创建数据库
createdb -h <主机> -p 5432 -U postgres refreshing

# 数据库表结构在首次启动时自动迁移，无需手动执行
```

### 4. 启动后端

```bash
# 安装 Go 依赖
go mod tidy

# 生成 Swagger 接口文档
make swagger

# 启动 HTTP API 服务器
go run main.go api
```

> 后端也支持独立运行 `scheduler` 和 `worker` 进程来处理异步任务：
> ```bash
> go run main.go scheduler   # 定时任务调度器
> go run main.go worker      # Asynq 任务处理工作进程
> ```

### 5. 启动前端

```bash
cd frontend

# 安装依赖
pnpm install

# 启动开发服务器（Turbopack）
pnpm dev
```

### 6. 访问应用

| 服务 | 地址 |
|------|------|
| 前端界面 | http://localhost:3000 |
| Swagger 接口文档 | http://localhost:8000/swagger/index.html |
| 健康检查 | http://localhost:8000/api/health |

## ⚙️ 配置说明

主要配置项（完整说明请参考 `config.example.yaml`）：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| `app.addr` | 后端监听地址 | `:8000` |
| `database.host` | PostgreSQL 主机 | `127.0.0.1` |
| `database.database` | 数据库名称 | `refreshing` |
| `redis.host` | Redis 主机 | `127.0.0.1` |
| `storage.endpoint` | S3 兼容存储端点 | `s3.amazonaws.com` |

## 🔧 开发指南

### 后端

```bash
# 运行 API 服务器
go run main.go api

# 运行定时任务调度器
go run main.go scheduler

# 运行异步任务工作进程
go run main.go worker

# 修改 Controller 后重新生成 Swagger 文档（必须执行）
make swagger

# 代码格式化与检查
make tidy
```

### 前端

```bash
cd frontend

# 开发模式（Turbopack）
pnpm dev

# 构建生产版本
pnpm build

# 启动生产服务器
pnpm start

# 代码 Lint 和格式化
pnpm lint
pnpm format
```

## 📁 项目结构

```
wavelet/
├── main.go                  # 程序入口（委托给 internal/cmd）
├── config.example.yaml      # 配置模板
├── Makefile                 # 常用命令（swagger、tidy、license、cross-build）
├── docker/                  # Docker 镜像构建文件（集成/前端/后端）
├── docs/                    # Swagger 自动生成文档
├── frontend/                # Next.js 前端应用
│   ├── app/                 # App Router 页面
│   ├── components/          # React 组件（ui、common、layout）
│   ├── lib/services/        # API 服务层
│   └── types/               # TypeScript 类型定义
└── internal/                # Go 后端（private）
    ├── cmd/                 # CLI 命令（api、scheduler、worker）
    ├── apps/                # 业务模块（oauth、user、admin、upload）
    ├── model/               # GORM 实体与业务方法
    ├── router/              # HTTP 路由注册
    ├── task/                # 异步任务定义与工作进程
    ├── db/                  # 数据库与 Redis 初始化
    ├── storage/             # S3 文件存储抽象层
    └── common/              # 公共工具与响应封装
```

## 📚 接口文档

Swagger 接口文档在后端启动后自动可用：

```
http://localhost:8000/swagger/index.html
```

前端文档中心（路径 `/docs`）内置以下内容：
- **使用指南** — 分步入门教程
- **接口文档** — 详细接口说明
- **隐私政策** — 隐私政策模板（请按需自定义）
- **服务条款** — 服务条款模板

## 🧪 测试

```bash
# 后端测试
go test ./...

# 前端 Lint
cd frontend && pnpm lint
```

## 🚀 部署

### 跨平台二进制编译

一条命令构建全部 6 个平台的静态二进制文件（Linux / macOS / Windows × amd64 / arm64）。
前端已内嵌到每个二进制文件中，无需单独部署。

**前提条件：** 已安装 Docker 且启用 BuildKit（Docker 23+ 默认开启）。

```bash
# 构建全部 6 个二进制文件 → ./bin/
make cross-build

# 指定版本号
make cross-build VERSION=v1.2.3

# 只构建指定系统（两种架构均会构建）
make cross-build GOOS=linux
make cross-build GOOS=darwin
make cross-build GOOS=windows

# 只构建指定架构（所有系统均会构建）
make cross-build GOARCH=amd64
make cross-build GOARCH=arm64

# 同时指定系统和架构 — 只生成单个文件
make cross-build GOOS=linux GOARCH=arm64
make cross-build GOOS=darwin GOARCH=amd64 VERSION=v1.2.3
```

输出到 `./bin/` 目录：

| 文件名 | 平台 |
|--------|------|
| `wavelet_linux_amd64` | Linux x86-64 |
| `wavelet_linux_arm64` | Linux ARM64 |
| `wavelet_darwin_amd64` | macOS Intel |
| `wavelet_darwin_arm64` | macOS Apple Silicon |
| `wavelet_windows_amd64.exe` | Windows x86-64 |
| `wavelet_windows_arm64.exe` | Windows ARM64 |

> 版本号可通过 `wavelet --version` 在运行时查看。

### Docker

```bash
# 构建镜像
docker build -t refreshing .

# 运行（通过卷挂载传入配置文件）
docker run -d -p 8000:8000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  refreshing api
```

### 生产环境

1. 构建前端资源：
   ```bash
   cd frontend && pnpm build
   ```

2. 编译后端程序：
   ```bash
   go build -o refreshing main.go
   ```

3. 配置生产环境的 `config.yaml`。

4. 启动服务：
   ```bash
   ./refreshing api        # HTTP API
   ./refreshing scheduler  # 定时调度器（可选）
   ./refreshing worker     # 任务工作进程（可选）
   ```

## 🤝 贡献指南

我们欢迎社区贡献！请在提交代码前阅读以下文档：

- [贡献指南](CONTRIBUTING.md)
- [行为准则](CODE_OF_CONDUCT.md)
- [贡献者许可协议](CLA.md)

### 贡献流程

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/your-feature`)
3. 提交更改 (`git commit -am 'Add your feature'`)
4. 推送到分支 (`git push origin feature/your-feature`)
5. 创建 Pull Request

## 📄 许可证

本项目基于 [Apache 2.0 许可证](LICENSE) 开源。
