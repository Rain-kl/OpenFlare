# 命令与脚本

你会学到：OpenFlare Server、管理端前端、Agent、Swagger 和文档站的常用启动、构建、测试、安装与卸载命令。

## Server

源码启动：

```bash
cd openflare-server
cp config.example.yaml config.yaml
go run . all
```

指定监听端口与日志目录：

```bash
go run . --port 3000 --log-dir ./logs
```

测试：

```bash
cd openflare-server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

## Frontend

开发：

```bash
cd openflare-server/frontend
pnpm install
pnpm dev
```

构建静态产物：

```bash
cd openflare-server/frontend
pnpm build:embed
```

检查：

```bash
cd openflare-server/frontend
pnpm lint
pnpm typecheck
pnpm test
```

## Agent

源码运行：

```bash
cd openflare-agent
go run ./cmd/agent -config /path/to/agent.json
```

编译：

```bash
cd openflare-agent
go build -o openflare-agent ./cmd/agent
```

测试：

```bash
cd openflare-agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

## Relay (中继端)

源码运行：

```bash
cd openflare-relay
go run ./cmd -config /path/to/relay.json
```

编译：

```bash
cd openflare-relay
go build -o openflare-relay ./cmd
```

## OpenFlared (Client 客户端)

源码运行：

```bash
cd openflared
go run ./cmd -config /path/to/flared.json
```

编译：

```bash
cd openflared
go build -o openflared ./cmd
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
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd openflare-server
swag init -g main.go -o docs
```

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
