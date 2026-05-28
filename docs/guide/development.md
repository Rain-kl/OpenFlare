# 本地开发

你会学到：如何搭建 OpenFlare 的本地开发环境、启动 Server、Agent 和管理端前端，运行测试与构建命令，并理解贡献代码前需要遵守的边界。

本页面向贡献者。产品边界、数据模型约束、API 约定和前端分层规范以 [开发约束](../design/development.md) 为准；本页只提供可执行的本地开发流程。

## 仓库结构

| 路径 | 职责 |
| --- | --- |
| `openflare_server` | Gin + GORM + SQLite/PostgreSQL 单体控制面 |
| `openflare_server/web` | Next.js 管理端前端，静态导出后由 Go Server 托管 |
| `openflare_agent` | Go 单体 Agent，运行在节点侧 |
| `scripts` | Agent 安装与卸载脚本 |
| `docs` | VitePress 文档站 |

## 环境要求

| 项目 | 要求 |
| --- | --- |
| Go | `1.25+` |
| Node.js | `18+` |
| pnpm | 推荐通过 `corepack enable` 使用项目声明版本 |
| Docker | Server 容器、本地联调和 Agent Docker 镜像需要 |
| OpenResty | 本地运行 Agent 时需要可执行 `openresty` |
| PostgreSQL | 可选；未配置时 Server 使用 SQLite |

## 初始化前端依赖

```bash
cd openflare_server/web
corepack enable
pnpm install
```

构建供 Go Server 托管的静态产物：

```bash
pnpm build
```

## 启动 Server

SQLite 模式：

```bash
cd openflare_server
export SESSION_SECRET='dev-session-secret'
export SQLITE_PATH='./openflare-dev.db'
export LOG_LEVEL='debug'
go run .
```

PostgreSQL 模式：

```bash
cd openflare_server
export SESSION_SECRET='dev-session-secret'
export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
export LOG_LEVEL='debug'
go run .
```

默认访问地址：

```text
http://localhost:3000
```

默认账号是 `root` / `123456`。

## 启动前端开发服务器

前端开发服务器默认监听 `3001`，并通过 `NEXT_DEV_BACKEND_URL` 代理到后端：

```bash
cd openflare_server/web
export NEXT_DEV_BACKEND_URL='http://127.0.0.1:3000'
pnpm dev
```

访问：

```text
http://localhost:3001
```

## 启动 Agent

创建本地 `agent.json`：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

运行：

```bash
cd openflare_agent
export LOG_LEVEL='debug'
go run ./cmd/agent -config ./agent.json
```

未配置 `openresty_path` 时，Agent 默认调用 `openresty`。调试时可显式配置 `openresty_path`、`main_config_path`、`route_config_path`、`access_log_path`、`cert_dir`、`lua_dir` 和 `runtime_config_dir`。

## 测试

Server：

```bash
cd openflare_server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Agent：

```bash
cd openflare_agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Frontend：

```bash
cd openflare_server/web
pnpm lint
pnpm typecheck
pnpm test
pnpm test:e2e
```

Docs：

```bash
cd docs
pnpm build
```

## 构建

管理端静态产物：

```bash
cd openflare_server/web
pnpm build
```

Server 二进制：

```bash
cd openflare_server
go build -o openflare-server .
```

Agent 二进制：

```bash
cd openflare_agent
go build -o openflare-agent ./cmd/agent
```

## 调试入口

| 场景 | 命令或位置 |
| --- | --- |
| Server 日志 | `LOG_LEVEL=debug go run .` |
| Agent 日志 | `LOG_LEVEL=debug go run ./cmd/agent -config ./agent.json` |
| Swagger | `http://localhost:3000/swagger/index.html` |
| 前端 API 代理 | `NEXT_DEV_BACKEND_URL=http://127.0.0.1:3000 pnpm dev` |
| OpenResty 配置校验 | `openresty -t -c ./data/etc/nginx/nginx.conf` |

## 代码风格与变更准入

贡献前先确认：

1. 需求符合 [产品边界](../design/index.md)。
2. 实现符合 [开发约束](../design/development.md)。
3. 不破坏发布、同步、回滚或升级主链路。
4. 涉及配置、部署、API 或产品边界时同步更新文档。
5. 风险较高的修改补充测试或等效联调验证。

数据库结构变更必须提升数据库版本号，并补充从上一版本到新版本的显式迁移方法和校验逻辑。
