# 命令与脚本

你会学到：OpenFlare Server、管理端前端、Agent、Relay、OpenFlared、Swagger 和文档站的常用启动、构建、测试、安装与卸载命令。

> 所有命令均在**仓库根目录**执行，除非另有说明。

## Server

源码启动：

```bash
cp config.example.yaml config.yaml
go run main.go all
```

分进程启动：

```bash
go run main.go api          # 仅 HTTP API
go run main.go worker       # 仅 Asynq Worker
go run main.go scheduler    # 仅定时任务调度
```

编译二进制：

```bash
make build-server
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

自动格式化后端 Go 与前端 TypeScript、JavaScript、CSS 等源码：

```bash
make prettier
```

该命令使用 `gofmt` 格式化后端，并使用项目固定版本的 Prettier 格式化 `frontend/` 源码；构建产物、依赖、公开静态资源和锁文件会被忽略。

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
pnpm typecheck
```

## Agent

源码运行：

```bash
go run ./cmd/agent -config /path/to/agent.json
```

编译：

```bash
make build-agent
# 或：go build -o bin/openflare-agent ./cmd/agent
```

测试：

```bash
GOCACHE=/tmp/openflare-go-cache go test ./internal/apps/agent/...
```

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

## 安装 Agent

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

## 卸载 Agent

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

## Swagger

重新生成 Swagger 文档：

```bash
make swagger
```

访问：`http://localhost:3000/swagger/index.html`

## Docs

本地预览：

```bash
cd docs
pnpm dev
```

构建：

```bash
cd docs
pnpm build:embed
```
