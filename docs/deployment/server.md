# Server 部署

OpenFlare Server 是基于 Gin 与 GORM 开发的单体控制面服务，负责管理端 Web 界面、管理 API、边缘节点通信 API、站点配置渲染与下发、异步任务调度及聚合查询。

> [!IMPORTANT]
> **外部依赖要求**：
> OpenFlare 内建基于 Asynq 框架的后台异步任务引擎。**无论采用何种部署模式，系统均必须依赖 Redis（或 Valkey）**。关系型主数据库支持 SQLite（轻量单机）与 PostgreSQL（生产标准）。业务访问日志规模较大时，推荐接入 ClickHouse 存储分析数据。

> [!TIP]
> **ClickHouse 性能优化配置（推荐挂载）**  
> 控制面宿主机通常配置较精简。官方仓库提供的 `performance.xml` 收紧了后台 merge/mutation 线程池，可避免默认配置在低规格实例上静置 CPU 偏高或 ClickHouse 25.x 启动校验不通过。  
> 挂载配置方式：将本地 `./config/clickhouse/performance.xml` 以单文件只读形式挂载至容器 `/etc/clickhouse-server/config.d/performance.xml`。

拉取 ClickHouse 优化配置：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
```

在 ClickHouse 服务的 `volumes` 中配置只读挂载：

```yaml
volumes:
  - ./data/clickhouse_data:/var/lib/clickhouse
  - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
```

修改配置后需执行 `docker compose restart clickhouse` 生效。

---

## 节点远程执行

远程执行默认关闭。需要下发 Shell 命令时，在 Server 的 `config.yaml` 顶层设置 `enable_webshell: true`，或为 Server 进程设置环境变量 `ENABLE_WEBSHELL=true`。环境变量优先。此配置仅在进程启动时读取，修改后须重启 Server。多实例部署中，处理请求和连接目标节点的 Server 实例都须启用该配置。

关闭时，控制台、CLI 和 MCP 均无法下发新命令，仍可查看已有执行记录。

## Docker Compose 部署方案

根据业务规模与数据存储需求，选择对应的编排方案：

### 方案 1：轻量单机方案（SQLite + Redis）

适用于测试体验、本地联调或轻量单机部署。主数据库使用嵌入式 SQLite。

创建 `docker-compose.yaml` 文件：

```yaml
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
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # 生产环境请使用强随机字符串
      DB_ENABLED: "false" # 禁用外部 PostgreSQL，启用内置 SQLite
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

启动命令：

```bash
docker compose up -d
docker compose ps
```

---

### 方案 2：标准生产方案（PostgreSQL + Redis）

适用于标准生产环境，提供可靠的关系型数据持久化与事务支持。

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

配置环境变量文件 `.env`：

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 文件，设置强随机密钥 APP_SESSION_SECRET 与数据库密码
docker compose up -d
```

---

### 方案 3：全功能可观测集群（含 ClickHouse 与 Jaeger）

在大流量业务场景下，推荐使用 ClickHouse 独立存储分析明细，并联动 Jaeger 收集 OpenTelemetry (OTel) 链路调用指标。

创建 `docker-compose.yaml` 文件：

```yaml
services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    env_file: .env
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://jaeger:4317"
      OTEL_EXPORTER_OTLP_INSECURE: "true"
      OTEL_SAMPLING_RATE: "1.0"
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

初始化与启动：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 配置各组件密码与 APP_SESSION_SECRET
docker compose up -d
```

启动后访问 `http://localhost:16686` 可打开 Jaeger 监控面板查看 Trace 调用链路。

### 自定义 OpenTelemetry 接收端

链路追踪默认使用 OTLP/gRPC 协议。若接收端使用 OTLP/HTTP protobuf，可在 Server 环境变量中指定：

```yaml
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT: https://collector.example.com
OTEL_SAMPLING_RATE: "0.1"
```

标准 SDK 会在通用 `OTEL_EXPORTER_OTLP_ENDPOINT` 路径后自动追加 `/v1/traces`。若接收端使用非标准路径，请直接配置 `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`。链路专用的 `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` 优先级高于通用协议配置；未配置时使用 `otel.export_protocol`（默认为 `grpc`）。

配置鉴权请求头：使用 `OTEL_EXPORTER_OTLP_HEADERS`（格式为 `key=value,key2=value2`，特殊字符需百分号编码，例如 `X-API-Key=example-key`）。链路专用 `OTEL_EXPORTER_OTLP_TRACES_HEADERS` 优先于通用头配置。

---

## 首次访问与认证

Server 默认监听 `3000` 端口。容器或服务启动后，在浏览器访问 `http://localhost:3000`。

初始管理员账密：

| 用户名 | 初始密码 |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> 首次登录后，请立即前往「个人中心 / 安全设置」修改默认密码。

---

## 分布式进程拆分

在大型集群或高可用部署中，可将 Server 按职责拆分为独立进程运行：

```bash
# 1. 仅启动管理端 UI 与节点通信的 HTTP API 服务
go run ./cmd/server api

# 2. 仅启动处理 Asynq 异步任务队列的 Worker 节点
go run ./cmd/server worker

# 3. 仅启动 Cron 定时任务的 Scheduler 调度器
go run ./cmd/server scheduler
```
