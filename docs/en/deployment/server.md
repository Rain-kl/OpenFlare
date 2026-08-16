# Start the Server

You will learn: how to deploy with Docker (quick start, production-recommended, advanced) and how to deploy the OpenFlare Server locally from source.

The OpenFlare Server is a Gin + GORM monolithic control plane responsible for the admin UI, admin API, Agent API, config rendering, version release, data storage, and aggregation queries.

> [!IMPORTANT]
> **About external dependencies**:
> OpenFlare has built-in support for background async tasks (Asynq framework). Therefore, **regardless of deployment mode, Redis (or Valkey) is required**. The main difference between deployment options is the primary relational DB choice (SQLite vs PostgreSQL) and whether tracing (Jaeger) is enabled.
> For high business traffic, ClickHouse is recommended for log storage.

> [!TIP]
> **ClickHouse server performance config (recommended mount)**
> The control plane is typically a small host (e.g. 3c6g). The `performance.xml` provided in the repo tightens the background merge/mutation thread pools, avoiding high idle CPU or ClickHouse 25.x startup validation failures on small machines.
> Mount the local `./config/clickhouse/performance.xml` as a single file at `/etc/clickhouse-server/config.d/performance.xml` to keep the official image's built-in Docker network listening config.

Pull the config locally before deploying:

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
```

Add it to the ClickHouse service `volumes` (alongside the data volume):

```yaml
volumes:
  - ./data/clickhouse_data:/var/lib/clickhouse   # or named volume
  - ./config/clickhouse/performance.xml:/etc/clickhouse-server/config.d/performance.xml:ro
```

After modifying `performance.xml`, run `docker compose restart clickhouse` for it to take effect.

---

## Method 1: Docker Deployment (recommended)

Docker deployment avoids configuring Go and Node.js frontend build environments locally. Choose one of the three options based on your hardware and needs:

### 1. Quick Start (SQLite + Redis)

> **Use case**: testing/experience, lightweight single-machine deployment.
>
> **Features**: primary relational DB is SQLite.

Create a `docker-compose.yaml`:

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
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # replace with a long random string in production
      DB_ENABLED: "false" # disables PostgreSQL, auto-enables the built-in SQLite fallback
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

### 2. Small-Traffic Business (PostgreSQL + Redis)

> **Use case**: production, small-to-medium traffic; PostgreSQL won't be the log-write bottleneck.

Create a `docker-compose.yaml`:

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

Create a matching `.env` file for system env vars (copy and modify the root `.env.example`):

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# edit .env: fill in DB, Redis, passwords, and APP_SESSION_SECRET

docker compose up -d
```

---

### 3. Advanced (full orchestration with Jaeger tracing)

> **Use case**: high traffic; needs trace performance metrics.
>
> **Features**: on top of the "production-recommended" bundle, stores logs with ClickHouse and uses Jaeger as the OpenTelemetry (OTel) tracing backend.

Create a `docker-compose.yaml`:

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
      OTEL_SAMPLING_RATE: "1.0" # sampling rate; 1.0 samples all traces
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
      - "16686:16686" # Web UI port
      - "4317:4317"   # OTLP gRPC receive port
      - "4318:4318"   # OTLP HTTP receive port

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

Start and verify:

```bash
mkdir -p ./config/clickhouse
curl -fsSL -o ./config/clickhouse/performance.xml \
  https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/config/clickhouse/performance.xml
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# edit .env and make sure APP_SESSION_SECRET password is set

docker compose up -d
```
After startup, open `http://localhost:16686` to view the Jaeger monitoring UI and system span traces.

---

## First Login

The Server listens on port `3000` by default; open `http://localhost:3000` in a browser after startup.

Default admin account:

| Username | Password |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> For your system's security, change the default password immediately in your profile settings after the first login.

---

## Distributed Deployment

In large production deployments, split the Server into multiple processes by responsibility:

```bash
go run main.go api             # API service for admin panel and node communication only
go run main.go worker          # background task Worker service only
go run main.go scheduler       # scheduled task Scheduler service only
```
