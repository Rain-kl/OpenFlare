# 启动 Server

你会学到：如何使用 Docker（分为快速启动、生产推荐、进阶版）部署，以及如何从源码本地部署 OpenFlare Server。

OpenFlare Server 是 Gin + GORM 单体控制面，负责管理端 UI、管理 API、Agent API、配置渲染、版本发布、数据存储与聚合查询。

> [!IMPORTANT]
> **关于外部依赖**：
> OpenFlare 系统内建了对后台异步任务（Asynq 框架）的支持。因此，**无论采用何种部署模式，系统都必须依赖 Redis（或 Valkey）**。各个部署方案的主要差异在于主关系型数据库的选择（SQLite vs PostgreSQL）以及是否启用链路追踪服务（Jaeger）。
> 若业务流量过大，建议使用 ClickHouse 存储日志。

> [!TIP]
> **ClickHouse 服务端性能配置（推荐挂载）**  
> 控制面常见为小规格主机（如 3c6g）。仓库提供的 `performance.xml` 会收紧后台 merge/mutation 线程池，避免默认配置在小机器上静置 CPU 偏高或 ClickHouse 25.x 启动校验失败。  
> 将本地 `./config/clickhouse/performance.xml` 以单文件方式挂载到容器 `/etc/clickhouse-server/config.d/performance.xml`，以保留官方镜像内置的 Docker 网络监听配置。

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
  - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
```

修改 `performance.xml` 后需 `docker compose restart clickhouse` 才生效。

---

## 方式一：Docker 部署（推荐）

使用 Docker 部署可以免去本地配置 Go 与 Node.js 前端构建环境的麻烦。根据你的服务器硬件配置及业务需求，你可以选择以下三种方案之一：

### 1. 快速启动（SQLite + Redis）

> **适用场景**：测试体验、轻量化单机部署。
>
> **特点**：主关系型数据库使用 SQLite 

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
    depends_on:
      redis:
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
```

---

### 2. 小流量业务场景（PostgreSQL + Redis）

> **适用场景**：生产环境、业务流量中小，PostgreSQL 不会成为日志记录的瓶颈。

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

volumes:
    openflare_uploads:
    openflare_postgres_data:
    openflare_redis_data:
```

创建对应的 `.env` 文件来配置系统环境变量（可复制并修改根目录下的 `.env.example`）：

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 文件，填入对应的数据库、Redis、密码与 APP_SESSION_SECRET

docker compose up -d
```

---

### 3. 进阶版（含 Jaeger 链路追踪的完整编排）

> **适用场景**：大流量场景，需要进行链路性能指标追踪。
>
> **特点**：在“生产推荐”全家桶的基础上，使用 ClickHouse 存储日志，联动 Jaeger 作为 OpenTelemetry (OTel) 链路追踪的后端。

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
      OTEL_SAMPLING_RATE: "1.0" # 采样率，1.0 表示采样全部 Trace
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
      - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
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

## 首次登录

Server 默认监听 `3000` 端口，启动成功后可以使用浏览器访问：`http://localhost:3000`。

默认管理员账户信息如下：

| 用户名 | 密码 |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> 为了你的系统安全，首次登录后请立即前往个人设置页面修改默认密码。

---

## 分布式部署

在大型生产部署中，你可以选择将 Server 按职责拆分为多个进程运行：

```bash
go run main.go api             # 仅启动管理端与节点通信的 API 服务
go run main.go worker          # 仅启动后台任务的 Worker 服务
go run main.go scheduler       # 仅启动定时任务的 Scheduler 服务
```
