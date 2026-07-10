# 启动 Server

你会学到：如何使用 Docker（分为快速启动、生产推荐、进阶版）部署，以及如何从源码本地部署 OpenFlare Server。

OpenFlare Server 是 Gin + GORM 单体控制面，负责管理端 UI、管理 API、Agent API、配置渲染、版本发布、数据存储与聚合查询。

> [!IMPORTANT]
> **关于外部依赖**：
> OpenFlare 系统内建了对后台异步任务（Asynq 框架）及海量节点日志分析与度量指标（观测面板）的支持。因此，**无论采用何种部署模式，系统都必须依赖 Redis（或 Valkey）与 ClickHouse 的运行**。各个部署方案的主要差异在于主关系型数据库的选择（SQLite vs PostgreSQL）以及是否启用链路追踪服务（Jaeger）。

> [!TIP]
> **ClickHouse 服务端性能配置（推荐挂载）**  
> 控制面常见为小规格主机（如 3c6g）。仓库提供的 `performance.xml` 会收紧后台 merge/mutation 线程池，避免默认配置在小机器上静置 CPU 偏高或 ClickHouse 25.x 启动校验失败。  
> 将本地目录 `./config/clickhouse` 挂载到容器 `/etc/clickhouse-server/config.d`。  
> **目录内只放 `performance.xml`，不要放入任何 listen 相关配置**（监听地址沿用官方镜像默认即可）。

部署前将配置拉到本地：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
```

在 ClickHouse 服务的 `volumes` 中增加（与数据卷并列）：

```yaml
volumes:
  - ./data/clickhouse_data:/var/lib/clickhouse   # 或 named volume
  - ./config/clickhouse:/etc/clickhouse-server/config.d:ro
```

修改 `performance.xml` 后需 `docker compose restart clickhouse` 才生效。

---

## 方式一：Docker 部署 (推荐)

使用 Docker 部署可以免去本地配置 Go 与 Node.js 前端构建环境的麻烦。根据你的服务器硬件配置及业务需求，你可以选择以下三种方案之一：

### 1. 快速启动 (SQLite + Redis + ClickHouse)

> **适用场景**：测试体验、轻量化单机部署。
>
> **特点**：主关系型数据库使用内建的 SQLite 文件

创建 `docker-compose.yaml` 文件：

```yaml
version: '3.8'

services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    container_name: openflare-server
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - ./openflare-data:/data
      - ./uploads:/app/uploads
    environment:
      TZ: Asia/Shanghai
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # 生产环境请替换为长随机字符串
      DB_ENABLED: "false" # 禁用 PostgreSQL，自动启用内置 SQLite 后备
      SQLITE_PATH: "/data/openflare.db"
      REDIS_ENABLED: "true"
      REDIS_ADDR: "redis:6379"
      CLICKHOUSE_ENABLED: "true"
      CLICKHOUSE_HOST: "clickhouse:9000"
    depends_on:
      redis:
        condition: service_healthy
      clickhouse:
        condition: service_healthy

  redis:
    image: valkey/valkey:8.0-alpine
    restart: unless-stopped
    command: ["valkey-server", "--appendonly", "yes"]
    volumes:
      - ./data/valkey:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  clickhouse:
    image: clickhouse/clickhouse-server:25.3-alpine
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: openflare
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: 123456
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      TZ: Asia/Shanghai
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    volumes:
      - ./data/clickhouse_data:/var/lib/clickhouse
      - ./config/clickhouse:/etc/clickhouse-server/config.d:ro
    healthcheck:
      test: ["CMD", "clickhouse-client", "--user", "default", "--password", "123456", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s
```

运行启动命令：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
docker compose up -d
```

---

### 2. 生产推荐 (PostgreSQL + Redis + ClickHouse)

> **适用场景**：生产环境、多节点集群管理、高并发高可用要求。
>
> **特点**：完全分层架构。启用专用的 PostgreSQL 服务作为主关系数据库，Redis 负责高并发分布式锁、会话缓存与异步队列，ClickHouse 承载海量日志异步 Flush 与观测指标。

创建 `docker-compose.yaml` 文件：

```yaml
services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    env_file: .env
    environment:
      TZ: ${TZ:-Asia/Shanghai}
    ports:
      - "3000:3000"
    volumes:
      - openflare_uploads:/app/uploads
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      clickhouse:
        condition: service_healthy

  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openflare}
      POSTGRES_USER: ${DB_USERNAME:-openflare}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-replace-with-strong-password}
    volumes:
      - openflare_postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USERNAME:-openflare} -d ${DB_NAME:-openflare}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: valkey/valkey:8.0-alpine
    restart: unless-stopped
    command: ["valkey-server", "--appendonly", "yes"]
    volumes:
      - openflare_redis_data:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s

  clickhouse:
    image: clickhouse/clickhouse-server:25.3-alpine
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_NAME:-openflare}
      CLICKHOUSE_USER: ${CLICKHOUSE_USERNAME:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      TZ: ${TZ:-Asia/Shanghai}
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    volumes:
      - openflare_clickhouse_data:/var/lib/clickhouse
      - ./config/clickhouse:/etc/clickhouse-server/config.d:ro
    healthcheck:
      test: ["CMD", "clickhouse-client", "--user", "${CLICKHOUSE_USERNAME:-default}", "--password", "${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s

volumes:
    openflare_uploads:
    openflare_postgres_data:
    openflare_redis_data:
    openflare_clickhouse_data:
```

创建对应的 `.env` 文件来配置系统环境变量（可复制并修改根目录下的 `.env.example`）：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 文件，填入对应的数据库、Redis、ClickHouse 连接地址、密码与 APP_SESSION_SECRET

docker compose up -d
```

---

### 3. 进阶版 (含 Jaeger 链路追踪的完整编排)

> **适用场景**：开发者调试、系统深度性能诊断、高级可观测性追溯。
>
> **特点**：在“生产推荐”全家桶的基础上，联动拉起 Jaeger 作为 OpenTelemetry (OTel) 链路追踪的后端，收集 Server 运行时各个 API 请求的 Span Trace 信息。

创建 `docker-compose.yaml` 文件：

```yaml
version: '3.8'

services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    env_file: .env
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://jaeger:4317"
      OTEL_EXPORTER_OTLP_INSECURE: "true"
      OTEL_SAMPLING_RATE: "1.0" # 本地调试建议设为 1.0 以采样所有 Trace
    ports:
      - "3000:3000"
    volumes:
      - openflare_uploads:/app/uploads
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      clickhouse:
        condition: service_healthy
      jaeger:
        condition: service_started

  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openflare}
      POSTGRES_USER: ${DB_USERNAME:-openflare}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-replace-with-strong-password}
    volumes:
      - openflare_postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USERNAME:-openflare} -d ${DB_NAME:-openflare}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: valkey/valkey:8.0-alpine
    restart: unless-stopped
    command: ["valkey-server", "--appendonly", "yes"]
    volumes:
      - openflare_redis_data:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s

  jaeger:
    image: jaegertracing/jaeger:2.19.0
    restart: unless-stopped
    environment:
      TZ: ${TZ:-Asia/Shanghai}
    ports:
      - "16686:16686" # Web UI 端口
      - "4317:4317"   # OTLP gRPC 接收端口
      - "4318:4318"   # OTLP HTTP 接收端口

  clickhouse:
    image: clickhouse/clickhouse-server:25.3-alpine
    restart: unless-stopped
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_NAME:-openflare}
      CLICKHOUSE_USER: ${CLICKHOUSE_USERNAME:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      TZ: ${TZ:-Asia/Shanghai}
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    volumes:
      - openflare_clickhouse_data:/var/lib/clickhouse
      - ./config/clickhouse:/etc/clickhouse-server/config.d:ro
    healthcheck:
      test: ["CMD", "clickhouse-client", "--user", "${CLICKHOUSE_USERNAME:-default}", "--password", "${CLICKHOUSE_PASSWORD:-replace-with-clickhouse-password}", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s

volumes:
  openflare_uploads:
  openflare_postgres_data:
  openflare_redis_data:
  openflare_clickhouse_data:
```

启动并验证：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 文件并确保设置好 APP_SESSION_SECRET 密码

docker compose up -d
```
启动后可以通过访问 `http://localhost:16686` 打开 Jaeger 监控端查看系统 Span 链路。

---

## 方式二：本地部署 (源码/二进制启动)

如果你不希望使用 Docker，也可以直接在本地或虚拟机上从源码构建和运行 Server。由于后台异步任务和可观测指标分析为系统核心防线，**本地部署时依然需要连接外部 Redis 与 ClickHouse 实例**。

### 前置条件

| 项目 | 要求 |
| --- | --- |
| Go | `1.25+` |
| Node.js | `18+` |
| pnpm | 推荐通过 `corepack enable` 使用项目声明的 pnpm |
| 外部服务 | 必须在本地或远端运行 Redis (Valkey) 和 ClickHouse 实例；ClickHouse 建议挂载仓库提供的 `performance.xml`（见上文「ClickHouse 服务端性能配置」） |

### 1. 构建管理端前端

Go Server 运行时需要嵌入前端静态资源。编译 Go 二进制前需要先构建前端静态产物并输出到 Go 服务目录：

```bash
cd frontend
corepack enable
pnpm install
pnpm build:embed
cd ..
```

> **常用前端代码检查命令**：
> * `pnpm lint`
> * `pnpm typecheck`

### 2. 使用 SQLite 启动

关系数据库存储在本地 SQLite 文件，但依然需要提供 Redis 和 ClickHouse 连接配置：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml：
# 1. 设置 app.session_secret 为一个随机的长字符串
# 2. 将 database.enabled 设为 false 以启用内置 SQLite
# 3. 将 redis.addrs 与 clickhouse.hosts 修改为你的本地/局域网服务连接信息

# 启动 Server（默认融合模式）
go run main.go all
```

### 3. 使用 PostgreSQL 启动

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml：
# 1. 设置 app.session_secret 
# 2. 将 database.enabled 设为 true，并完整设置 database.*、redis.*、clickhouse.* 字段连接参数

# 启动 Server（默认融合模式）
go run main.go all
```

---

## 首次登录

Server 默认监听 `3000` 端口，启动成功后可以使用浏览器访问：`http://localhost:3000`。

默认管理员账户信息如下：

| 用户名 | 密码 |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> 为了你的系统安全，首次登录后请立即前往个人设置页面修改默认密码。

---

## 常用运维指南

### 1. 命令行子服务分进程启动

在大型生产部署中，你可以选择将 Server 按职责拆分为多个进程运行：

```bash
go run main.go api       # 仅启动管理端与节点通信的 API 服务
go run main.go worker    # 仅启动后台任务的 Worker 服务
go run main.go scheduler # 仅启动定时任务的 Scheduler 服务
go run main.go all       # 融合模式（在一进程内运行上述所有服务，默认）
```

### 2. 状态验证

```bash
# 验证编译是否通过
go build ./...

# 运行内部单元测试
go test ./internal/apps/openflare/... -count=1

# 检查服务健康状态
curl http://127.0.0.1:3000/api/v1/d/status
```
