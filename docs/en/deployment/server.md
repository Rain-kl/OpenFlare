# Server deployment

OpenFlare Server is the control plane service managing the web console, Admin APIs, node protocols, site snapshot compilation, job scheduling, and metric aggregation.

> [!IMPORTANT]
> **Redis (or Valkey) is required in all deployment modes** for Asynq background jobs. The relational database supports SQLite for lightweight single-host setups and PostgreSQL for production; ClickHouse and Jaeger are optional for analytical logs and tracing.

## Docker Compose deployment profiles

### Profile 1: Single-node (SQLite and Redis)

Use this profile for local evaluation or lightweight single-host deployments. Create `docker-compose.yaml`:

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

Replace `APP_SESSION_SECRET` with a strong random secret before public exposure, then start:

```bash
docker compose up -d
docker compose ps
```

### Profile 2: Standard production (PostgreSQL and Redis)

Use PostgreSQL and Redis for production workloads. Create `docker-compose.yaml`:

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

Download the environment template, set credentials, and start:

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# Edit .env to set APP_SESSION_SECRET and DB_PASSWORD
docker compose up -d
```

### Profile 3: Full observability stack (ClickHouse and Jaeger)

For high-volume workloads, add ClickHouse for analytical logs and Jaeger for OpenTelemetry traces. The repository ships `config/clickhouse/performance.xml` to cap background merge/mutation threads on small hosts and satisfy ClickHouse 25.x startup checks. Create `docker-compose.yaml`:

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

Fetch the ClickHouse config, set `CLICKHOUSE_ENABLED=true`, passwords, and `APP_SESSION_SECRET` in `.env`, then start (Jaeger UI listens at `http://localhost:16686`):

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
docker compose up -d
```

### Custom OpenTelemetry exporters

Tracing defaults to OTLP/gRPC. To send traces via OTLP/HTTP protobuf:

```yaml
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT: https://collector.example.com
OTEL_SAMPLING_RATE: "0.1"
```

The SDK appends `/v1/traces` to `OTEL_EXPORTER_OTLP_ENDPOINT`; set `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` for custom paths. Pass collector authentication headers with `OTEL_EXPORTER_OTLP_HEADERS` (`key=value,key2=value2`, percent-encoded) or the higher-precedence `OTEL_EXPORTER_OTLP_TRACES_HEADERS`.

## Initial sign-in and remote execution

The Server listens on `:3000` by default (configurable via `app.addr` or `APP_ADDR`). Open `http://localhost:3000` and sign in with username `admin` and password `12345678`.

> [!WARNING]
> Change the default password under account security settings immediately after signing in.

Node remote execution is disabled by default. To allow dispatching shell commands from the console, CLI, or MCP, set `enable_webshell: true` at the top level of `config.yaml` or `ENABLE_WEBSHELL=true` and restart all Server instances; when disabled, historical execution logs remain readable.

## Source builds and process separation

Build the single binary with embedded frontend assets and run (Go version is defined in `go.mod`):

```bash
make build-embedded
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
./bin/openflare-server all
```

For multi-instance deployments, split the Server into dedicated processes:

```bash
./bin/openflare-server api        # HTTP API and console server
./bin/openflare-server worker     # Asynq background worker
./bin/openflare-server scheduler  # Cron task scheduler
```
