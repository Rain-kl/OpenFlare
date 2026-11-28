# Server 部署

OpenFlare Server 是控制面服务，负责控制台界面、管理 API、节点协议、站点快照编译、任务调度与观测聚合。

> [!IMPORTANT]
> **Redis（或 Valkey）为所有部署模式的必需依赖**。关系主库支持 SQLite（轻量单机）与 PostgreSQL（生产环境），分析日志与追踪可选接入 ClickHouse 与 Jaeger。

## Docker Compose 部署方案

### 方案 1：轻量单机（SQLite + Redis）

适用于测试或轻量单机环境。创建 `docker-compose.yaml`：

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
      APP_SESSION_SECRET: 'replace-with-a-long-random-string'
      DB_ENABLED: "false"
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

公网暴露前将 `APP_SESSION_SECRET` 替换为强随机密钥，然后启动：

```bash
docker compose up -d
docker compose ps
```

### 方案 2：标准生产（PostgreSQL + Redis）

适用于常规生产环境。创建 `docker-compose.yaml`：

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

下载环境变量模板、配置密钥与数据库密码并启动：

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env，设置 APP_SESSION_SECRET 与 DB_PASSWORD
docker compose up -d
```

### 方案 3：全功能可观测集群（ClickHouse + Jaeger）

大流量场景下接入 ClickHouse 存储访问日志与时序指标，并接入 Jaeger 收集 OpenTelemetry 链路追踪。仓库提供 `config/clickhouse/performance.xml` 以限制后台 merge/mutation 线程，避免低配主机静置 CPU 偏高或 ClickHouse 25.x 启动校验失败。创建 `docker-compose.yaml`：

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
      - "16686:16686"
      - "4317:4317"
      - "4318:4318"

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

下载配置、在 `.env` 中设置 `CLICKHOUSE_ENABLED=true`、密码及 `APP_SESSION_SECRET` 后启动（启动后在 `http://localhost:16686` 访问 Jaeger）：

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
docker compose up -d
```

### 自定义 OpenTelemetry 接收端

链路追踪默认使用 OTLP/gRPC；如需对接 OTLP/HTTP protobuf，设置以下环境变量：

```yaml
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT: https://collector.example.com
OTEL_SAMPLING_RATE: "0.1"
```

标准 SDK 会自动在 `OTEL_EXPORTER_OTLP_ENDPOINT` 后追加 `/v1/traces`；若接收端使用非标准路径，直接设置 `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`。鉴权请求头通过 `OTEL_EXPORTER_OTLP_HEADERS`（`key=value,key2=value2`，特殊字符需百分号编码）或更高优先级的 `OTEL_EXPORTER_OTLP_TRACES_HEADERS` 传入。

## 首次访问与节点远程执行

Server 默认监听 `:3000`（可通过 `app.addr` 或 `APP_ADDR` 修改）。启动后访问 `http://localhost:3000`，使用初始账号 `admin` 与密码 `12345678` 登录。

> [!WARNING]
> 首次登录后立即前往 **个人中心** > **安全设置** 修改默认密码。

节点远程执行默认关闭。如需从控制台、CLI 或 MCP 下发 Shell 命令，在 `config.yaml` 顶层设置 `enable_webshell: true` 或注入启动环境变量 `ENABLE_WEBSHELL=true` 后重启所有 Server 实例；关闭时仅保留历史记录查询。

## 源码编译与多进程拆分

从仓库源码构建嵌入前端资源的单一二进制并运行（Go 版本见根目录 `go.mod`）：

```bash
make build-embedded
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
./bin/openflare-server all
```

高可用或多实例场景下可将 Server 拆分为独立进程运行：

```bash
./bin/openflare-server api        # HTTP API 与控制台服务
./bin/openflare-server worker     # Asynq 异步任务消费
./bin/openflare-server scheduler  # Cron 定时调度器
```
