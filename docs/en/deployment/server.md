# Server deployment

OpenFlare Server is a monolithic control plane developed with Gin and GORM. It provides the web admin dashboard, administration APIs, edge node communication endpoints, configuration rendering and distribution, background asynchronous task scheduling, and metrics aggregation.

> [!IMPORTANT]
> **External dependencies**:
> OpenFlare uses the Asynq framework for background tasks. **Redis (or Valkey) is strictly required across all deployment modes**. The primary relational database supports SQLite (single-node) or PostgreSQL (standard production). For high-volume access logging, ClickHouse is recommended for analytical storage.

> [!TIP]
> **ClickHouse performance configuration (recommended mount)**  
> Control plane hosts often operate with moderate resource allocations. The repository-provided `performance.xml` restricts background merge/mutation thread pools, avoiding elevated idle CPU usage or ClickHouse 25.x startup verification failures.  
> Mount `./config/clickhouse/performance.xml` as a read-only single file at `/etc/clickhouse-server/config.d/performance.xml`.

Fetch the tuning configuration:

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
```

Mount it within the ClickHouse service volume section:

```yaml
volumes:
  - ./data/clickhouse_data:/var/lib/clickhouse
  - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
```

Apply changes by running `docker compose restart clickhouse`.

---

## Docker Compose deployment profiles

Select an orchestration profile matching your environment and traffic profile:

### Profile 1: Single-node (SQLite + Redis)

Recommended for evaluation, development, or single-node private environments. Uses embedded SQLite as the primary database.

Create `docker-compose.yaml`:

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
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # Must be a strong random secret in production
      DB_ENABLED: "false" # Disables PostgreSQL; activates built-in SQLite
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

Start the stack:

```bash
docker compose up -d
docker compose ps
```

---

### Profile 2: Standard production (PostgreSQL + Redis)

Recommended for standard production workloads, providing relational data persistence and ACID transaction support.

Create `docker-compose.yaml`:

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

Configure `.env`:

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# Edit .env and supply secure database credentials and APP_SESSION_SECRET
docker compose up -d
```

---

### Profile 3: Full observability stack (ClickHouse and Jaeger)

Recommended for high-throughput deployments requiring isolated analytical storage for access logs and Jaeger distributed tracing via OpenTelemetry (OTel).

Create `docker-compose.yaml`:

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
      - "16686:16686" # Web UI
      - "4317:4317"   # OTLP gRPC receiver
      - "4318:4318"   # OTLP HTTP receiver

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

Initialize and start:

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# Edit .env, configure credentials and APP_SESSION_SECRET
docker compose up -d
```

Access the Jaeger UI at `http://localhost:16686` to inspect trace spans.

### Custom OpenTelemetry exporters

Distributed tracing defaults to OTLP/gRPC. To use OTLP/HTTP protobuf, set the following environment variables:

```yaml
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT: https://collector.example.com
OTEL_SAMPLING_RATE: "0.1"
```

The standard SDK appends `/v1/traces` to `OTEL_EXPORTER_OTLP_ENDPOINT`. If your collector uses a non-standard path, set `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` directly. The trace-specific `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` takes precedence over general protocol settings.

Configure authentication headers via `OTEL_EXPORTER_OTLP_HEADERS` (format `key=value,key2=value2`, with special characters percent-encoded).

---

## Initial login and credentials

The Server listens on port `3000` by default. Once the container is running, open `http://localhost:3000` in your browser.

Default administrator credentials:

| Username | Default password |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> Change the default password in the admin console immediately upon first login.

---

## Distributed process separation

For large-scale deployments, the server can be split into dedicated processes:

```bash
# 1. HTTP API server for admin UI and node communication
go run ./cmd/server api

# 2. Worker node processing background Asynq tasks
go run ./cmd/server worker

# 3. Cron task scheduler
go run ./cmd/server scheduler
```
