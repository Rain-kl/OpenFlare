# OpenFlare 部署说明

本文档只保留 OpenFlare `1.0.0` 的当前部署基线、联调入口与升级方式。

## 1. 前置条件

### 1.1 Server

* Go 1.24+
* Node.js 18+
* 可写 SQLite 文件目录，或可访问的 PostgreSQL 实例

### 1.2 Agent

* Go 1.23+
* 对 Agent 数据目录有写权限
* 本机模式下可执行 `openresty -t` 与 `openresty -s reload`
* Docker 模式下具备 Docker 执行权限

## 2. 启动 Server

### 2.1 构建前端

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

`pnpm build` 会生成供 Go Server 托管的静态产物。

### 2.2 源码启动

```bash
cd openflare_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
# 可选：设置后优先使用 PostgreSQL。
# 如果 PostgreSQL 为空且本地 SQLite 文件存在，启动时会自动迁移数据。
# export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
go run .
```

默认监听 `3000` 端口。

### 2.3 Docker Compose 启动

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
    container_name: openflare
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-random-string
      SQLITE_PATH: /data/openflare.db
      DSN: postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info
    volumes:
      - openflare-data:/data

volumes:
  postgres-data:
  openflare-data:
```

```bash
docker compose up -d
```

### 2.4 首次登录

访问 `http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

### 2.5 Swagger

登录管理端后访问：`http://localhost:3000/swagger/index.html`

如需在本地重新生成文档：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
cd openflare_server
swag init -g main.go -o docs
```

## 3. Agent 配置

当前支持两种接入模式。

### 3.1 使用节点专属 `agent_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_container_name": "openflare-openresty",
  "openresty_docker_image": "openresty/openresty:alpine",
  "openresty_observability_port": 18081,
  "observability_replay_minutes": 15,
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

### 3.2 使用全局 `discovery_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "discovery_token": "replace-with-global-discovery-token",
  "data_dir": "./data",
  "openresty_container_name": "openflare-openresty",
  "openresty_docker_image": "openresty/openresty:alpine",
  "openresty_observability_port": 18081,
  "observability_replay_minutes": 15,
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

说明：

* `agent_token` 与 `discovery_token` 至少填写一个
* 未配置 `openresty_path` 时默认使用 Docker OpenResty
* Agent 会暴露本机观测端口并在 server 恢复后补传最近窗口数据

## 4. 启动 Agent

### 4.1 直接运行

```bash
cd openflare_agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

### 4.2 编译后二进制运行

```bash
cd openflare_agent
go build -o openflare-agent ./cmd/agent
export LOG_LEVEL='info'
./openflare-agent -config /path/to/agent.json
```

## 5. 最小联调步骤

1. 在管理端准备 `agent_token` 或 `discovery_token`
2. 启动 Agent 并确认节点上线
3. 新增一条启用中的反代规则
4. 生成并激活新版本
5. 确认 Agent 拉取配置、执行 `openresty -t`、reload 并上报结果

预期管理端可看到：

* 节点在线状态
* 节点当前版本
* 最近一次应用结果
* 自动注册后的专属 `agent_token`

## 6. 升级说明

* Root 用户可在管理端顶栏检查并升级 Server 正式版
* 如需尝试 preview 版本，可手动检查对应发布
* 节点 Agent 默认只跟随正式版自动更新；preview 升级需要手动触发
* 也可通过上传 Server 二进制的方式执行确认升级

## 7. 常用验证命令

### 7.1 Server

```bash
cd openflare_server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

### 7.2 Agent

```bash
cd openflare_agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

### 7.3 Frontend

```bash
cd openflare_server/web
pnpm build
```

## 8. Agent 一键部署

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

支持参数：

* `--server-url`
* `--discovery-token`
* `--agent-token`
* `--install-dir`
* `--repo`
* `--no-service`

安装脚本会下载最新 Agent、生成 `agent.json`、创建 `openflare-agent.service` 并启动服务。

## 9. 文档维护要求

部署方式、升级方式、接入模式或联调流程变化时，同步更新本文档和 `README.md`。
