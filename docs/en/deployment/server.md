# Launch Server

You will learn: How to build the admin frontend from source, start the OpenFlare Server, choose between SQLite or PostgreSQL, and access Swagger.

OpenFlare Server is a Gin + GORM monolithic control plane, responsible for managing the Admin UI, Admin API, Agent API, configuration rendering, version publishing, data storage, and aggregated queries.

## Prerequisites

| Item | Requirement |
| --- | --- |
| Go | `1.25+` |
| Node.js | `18+` |
| pnpm | Recommended enabling via `corepack enable` |
| Database | SQLite parent directory must be writable, or a reachable PostgreSQL instance |

In production environments, we highly recommend explicitly configuring `SESSION_SECRET` and prioritizing PostgreSQL.

## Build the Admin Frontend

The Go Server hosts static assets located in `openflare-server/web/build`. Before starting the Server from source, build the frontend:

```bash
cd openflare-server/web
corepack enable
pnpm install
pnpm build
```

Common frontend quality checks:

```bash
pnpm lint
pnpm typecheck
pnpm test
```

## Start with SQLite

```bash
cd openflare-server
export SESSION_SECRET='replace-with-a-long-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
go run .
```

By default, the Server listens on port `3000`. Access it at:

```text
http://localhost:3000
```

## Start with PostgreSQL

```bash
cd openflare-server
export SESSION_SECRET='replace-with-a-long-random-string'
export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
export LOG_LEVEL='info'
go run .
```

If `DSN` is set, it takes precedence over SQLite. When both `DSN` and the legacy `SQL_DSN` exist, `DSN` is prioritized.

If the target PostgreSQL database is empty and a local SQLite database exists at `SQLITE_PATH`, the Server automatically migrates the SQLite data into PostgreSQL during startup, outputting the migration progress in the logs.

## Start with Docker

Deploying with Docker avoids the hassle of setting up local Go and Node.js environments. OpenFlare provides official Dockerfiles and Compose configurations to support independent container startups and multi-service orchestrations.

### 1. Quick Start via Docker Run (SQLite Example)

Ensure that a local directory for persisting databases and logs has been created. Run the following command to start the Server:

```bash
# Create local mount directory
mkdir -p ./openflare-data

# Start the container
docker run -d \
  --name openflare-server \
  -p 3000:3000 \
  -v $(pwd)/openflare-data:/data \
  -e SESSION_SECRET='replace-with-a-long-random-string' \
  -e SQLITE_PATH='/data/openflare.db' \
  -e GIN_MODE='release' \
  -e LOG_LEVEL='info' \
  ghcr.io/rain-kl/openflare:latest
```

Startup parameters:
* **`-p 3000:3000`**: Maps port `3000` on the host to port `3000` inside the container.
* **`-v $(pwd)/openflare-data:/data`**: Mounts the local directory to `/data` in the container, ensuring that the SQLite database `openflare.db` is not lost when restarting or rebuilding the container.
* **`SESSION_SECRET`**: The session signing hash key (required).

---

### 2. One-click Startup via Docker Compose (Integrated PostgreSQL)

We recommend using Docker Compose in production environments to orchestrate an independent PostgreSQL database and establish high-availability relationships.

Create a `docker-compose.yml` file:

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
    image: ghcr.io/rain-kl/openflare:latest
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
      - ./openflare-data:/data
```

Start the services:

```bash
docker compose up -d
```

Compose configuration options:
* **`depends_on` and `healthcheck`**: Uses PostgreSQL's health check (`pg_isready`) to ensure that the database is fully initialized and ready before launching the OpenFlare Server, preventing panics from failed database connection attempts on first launch.
* **Separated Data Volume Mounts**: PostgreSQL data is mounted under `./postgres-data`, and OpenFlare data and backups are mounted under `./openflare-data`, making backups and maintenance simple.

## CLI Arguments

```bash
go run . --port 3000 --log-dir ./logs
```

| Argument | Description | Default Value |
| --- | --- | --- |
| `--port` | The port the Server listens to | `3000` |
| `--log-dir` | The directory to write logs to | Empty, outputs to stdout |
| `--version` | Outputs version and exits | `false` |
| `--help` | Outputs help and exits | `false` |

## First Login

Default credentials:

| Username | Password |
| --- | --- |
| `root` | `123456` |

Please change the default password immediately after your first login.
