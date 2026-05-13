# 部署说明

本文档说明 OpenFlare `1.0.0` 之后的部署基线、联调入口、升级方式与 Agent 一键部署流程。

## 前置条件

Server：

* Go 1.24+
* Node.js 18+
* 可写 SQLite 文件目录，或可访问的 PostgreSQL 实例

Agent：

* Go 1.24+
* 对 Agent 数据目录有写权限
* 本机模式下可执行 `openresty -t` 与 `openresty -s reload`
* Docker 模式下具备 Docker 执行权限

## Docker Compose 启动 Server

推荐生产部署使用 PostgreSQL：

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

首次访问 `http://localhost:3000`，默认账号为 `root` / `123456`。登录后请立即修改默认密码。

## 源码启动 Server

先构建管理端前端：

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

再启动 Server：

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

## Swagger

登录管理端后访问：

```text
http://localhost:3000/swagger/index.html
```

本地重新生成 Swagger：

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd openflare_server
swag init -g main.go -o docs
```

## Agent 接入模式

Agent 支持两种接入模式。

使用节点专属 `agent_token`：

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

使用全局 `discovery_token`：

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

* `agent_token` 与 `discovery_token` 至少填写一个。
* 未配置 `openresty_path` 时默认使用 Docker OpenResty。
* Agent 会暴露本机观测端口并在 Server 恢复后补传最近窗口数据。

## 一键部署 Agent

使用 `discovery_token`：

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

支持参数：

| 参数 | 说明 |
| --- | --- |
| `--server-url` | Server 地址 |
| `--discovery-token` | 首次自动注册 Token |
| `--agent-token` | 节点专属 Token |
| `--install-dir` | 安装目录 |
| `--repo` | 下载 Agent 的仓库 |
| `--no-service` | 不创建系统服务 |

安装脚本会下载最新 Agent、生成 `agent.json`、创建 `openflare-agent.service` 并启动服务。

## 手动启动 Agent

源码运行：

```bash
cd openflare_agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

编译后二进制运行：

```bash
cd openflare_agent
go build -o openflare-agent ./cmd/agent
export LOG_LEVEL='info'
./openflare-agent -config /path/to/agent.json
```

## 卸载 Agent

如需彻底卸载 Agent 并清空本地数据：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

支持参数：

| 参数 | 说明 |
| --- | --- |
| `--install-dir` | Agent 安装目录 |
| `--service-name` | systemd 服务名 |

卸载脚本会先停止 Agent、移除 `openflare-agent.service`、删除整个安装目录，再根据卸载前保存的 `agent.json` 判断 OpenResty 安装方式：

* Docker 模式：删除对应容器，并尝试移除 OpenResty 镜像。
* 本机 `openresty_path` 模式：不改动本机 OpenResty，仅提示用户手动卸载。

## 最小联调步骤

1. 在管理端准备 `agent_token` 或 `discovery_token`。
2. 启动 Agent 并确认节点上线。
3. 新增一条启用中的反代规则。
4. 生成并激活新版本。
5. 确认 Agent 拉取配置、执行 `openresty -t`、reload 并上报结果。

预期管理端可看到节点在线状态、节点当前版本、最近一次应用结果，以及自动注册后的专属 `agent_token`。

## 升级说明

* Root 用户可在管理端顶栏检查并升级 Server 正式版。
* 如需尝试 preview 版本，可手动检查对应发布。
* 节点 Agent 默认只跟随正式版自动更新；preview 升级需要手动触发。
* 也可通过上传 Server 二进制的方式执行确认升级。

## 常用验证命令

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
pnpm build
```
