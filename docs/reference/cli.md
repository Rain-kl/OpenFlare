# 命令与脚本

你会学到：OpenFlare Server、管理端前端、Agent、Relay、OpenFlared、Swagger 和文档站的常用启动、构建、测试、安装与卸载命令。

> 所有命令均在**仓库根目录**执行，除非另有说明。

## Server

源码启动：

```bash
cp config.example.yaml config.yaml
go run ./cmd/server all
```

分进程启动：

```bash
go run ./cmd/server api          # 仅 HTTP API
go run ./cmd/server worker       # 仅 Asynq Worker
go run ./cmd/server scheduler    # 仅定时任务调度
```

编译二进制：

```bash
make build-backend
# 产物：bin/openflare-server
```

测试：

```bash
GOCACHE=/tmp/openflare-go-cache go test ./...
```

质量门禁：

```bash
make code-check
```

自动格式化后端 Go 源码（整理导入）与前端源码：

```bash
make format
```

该命令使用 `goimports` 整理后端 Go 源码导入，并使用项目固定版本的 Prettier 格式化 `frontend/` 源码；构建产物、依赖、公开静态资源和锁文件会被忽略。

## Frontend

开发：

```bash
cd frontend
pnpm install
pnpm dev
```

构建嵌入产物（供 Go Server 托管）：

```bash
cd frontend
pnpm build:embed
# 或仓库根目录：make build-embedded
```

检查：

```bash
cd frontend
pnpm lint
pnpm tsc --noEmit --jsx preserve
pnpm check:i18n
```

## Pingora Edge

```bash
make build-edge
cargo test --manifest-path edge/pingora/Cargo.toml --locked
```

运行参数见 [Pingora Edge 启动参数](./configuration.md#pingora-edge-启动参数)。

## Relay（中继端）

源码运行：

```bash
go run ./cmd/relay -config /path/to/relay.json
```

编译：

```bash
make build-relay
# 或：go build -o bin/openflare-relay ./cmd/relay
```

## OpenFlared（Tunnel 客户端）

源码运行：

```bash
go run ./cmd/flared -config /path/to/flared.json
```

编译：

```bash
make build-flared
# 或：go build -o bin/flared ./cmd/flared
```

## 安装与卸载 Pingora Edge

安装说明见 [快速开始](../guide/quick-start.md#3-安装-pingora-edge)。卸载在边缘节点运行发行包中的 `scripts/uninstall-openflare-edge.sh`。

## Swagger

重新生成 Swagger 文档：

```bash
make swagger
```

访问：`http://localhost:3000/api/swagger/index.html`（默认 `api_prefix` 为 `/api`，仅非生产环境挂载）

## Docs

本地预览：

```bash
cd docs
pnpm dev
```

构建：

```bash
cd docs
pnpm build
```
