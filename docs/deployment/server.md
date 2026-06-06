# 启动 Server

你会学到：如何从源码构建管理端前端、启动 OpenFlare Server、选择 SQLite 或 PostgreSQL，并访问 Swagger。

OpenFlare Server 是 Gin + GORM 单体控制面，负责管理端 UI、管理 API、Agent API、配置渲染、版本发布、数据存储与聚合查询。

## 前置条件

| 项目 | 要求 |
| --- | --- |
| Go | `1.25+` |
| Node.js | `18+` |
| pnpm | 推荐通过 `corepack enable` 使用项目声明的 pnpm |
| 数据库 | SQLite 文件目录可写，或可访问的 PostgreSQL 实例 |

生产环境必须显式配置 `JWT_SECRET`，并优先使用 PostgreSQL。

## 构建管理端前端

Go Server 会托管 `openflare-server/web/build` 中的静态产物。源码启动前先构建前端：

```bash
cd openflare-server/web
corepack enable
pnpm install
pnpm build
```

常用前端检查：

```bash
pnpm lint
pnpm typecheck
pnpm test
```

## 使用 SQLite 启动

```bash
cd openflare-server
export JWT_SECRET='replace-with-a-long-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
go run .
```

默认监听 `3000` 端口，访问：

```text
http://localhost:3000
```

## 使用 PostgreSQL 启动

```bash
cd openflare-server
export JWT_SECRET='replace-with-a-long-random-string'
export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
export LOG_LEVEL='info'
go run .
```

`DSN` 设置后优先于 SQLite。`DSN` 与兼容旧命名的 `SQL_DSN` 同时存在时，优先使用 `DSN`。

如果目标 PostgreSQL 数据库为空且本地 `SQLITE_PATH` 文件存在，Server 启动阶段会尝试把 SQLite 数据迁移到 PostgreSQL，并在日志中输出迁移进度。

## 使用 Docker 启动

使用 Docker 部署可以免去本地配置 Go 与 Node.js 前端构建环境的麻烦。OpenFlare 官方提供了完整的 Dockerfile 与 Compose 配置，支持独立容器启动及多服务联动部署。

### 1. 使用 Docker Run 极速启动（以 SQLite 为例）

确保当前目录下已创建用于持久化数据库和日志的数据卷目录。运行以下命令启动 Server：

```bash
# 创建本地挂载目录
mkdir -p ./openflare-data

# 启动容器
docker run -d \
  --name openflare-server \
  -p 3000:3000 \
  -v $(pwd)/openflare-data:/data \
  -e JWT_SECRET='replace-with-a-long-random-string' \
  -e SQLITE_PATH='/data/openflare.db' \
  -e GIN_MODE='release' \
  -e LOG_LEVEL='info' \
  openflare-server:latest
```

启动参数说明：
* **`-p 3000:3000`**：映射宿主机 `3000` 端口到容器内 `3000` 端口。
* **`-v $(pwd)/openflare-data:/data`**：挂载本地目录到容器的 `/data`，确保数据库文件 `openflare.db` 在重启或重建容器时不丢失。
* **`JWT_SECRET`**：管理端 API 登录令牌的 JWT 签名密钥，生产环境必须配置，避免重启后已登录令牌全部失效。

---

### 2. 使用 Docker Compose 一键启动（集成 PostgreSQL）

推荐在生产环境使用 Docker Compose，自动编排独立的 PostgreSQL 数据库并建立服务间的高可用关联。

在项目控制面目录下使用 `docker-compose.yaml` 进行编排：

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
      - ./postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openflare -d openflare"]
      interval: 10s
      timeout: 5s
      retries: 5

  openflare:
    image: openflare-server:latest
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "3000:3000"
    environment:
      JWT_SECRET: replace-with-random-string
      SQLITE_PATH: /data/openflare.db
      DSN: postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info
    volumes:
      - ./openflare-data:/data
```

启动命令：

```bash
# 启动编排服务
docker compose up -d
```

Compose 参数说明：
* **`depends_on` 与 `healthcheck`**：通过 PostgreSQL 的健康度检查（pg_isready），确保数据库初始化完成并完全准备就绪后，再自动拉起 OpenFlare 控制面服务，避免首次连接数据库失败抛出 panic。
* **数据目录分离挂载**：`postgres` 数据挂载在 `./postgres-data`，`openflare` 数据与本地备份挂载在 `./openflare-data`，结构清晰，便于日常备份和维护。


## 命令行参数

```bash
go run . --port 3000 --log-dir ./logs
```

| 参数 | 作用 | 默认值 |
| --- | --- | --- |
| `--port` | 指定 Server 监听端口 | `3000` |
| `--log-dir` | 指定日志目录 | 空，输出到标准输出 |
| `--version` | 输出版本后退出 | `false` |
| `--help` | 输出帮助后退出 | `false` |

## 首次登录

默认账号：

| 用户名 | 密码 |
| --- | --- |
| `root` | `123456` |

首次登录后请立即修改默认密码。
