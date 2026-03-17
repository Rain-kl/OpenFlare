<p align="right">
  <a href="./README.md">中文</a> | <strong>English</strong>
</p>

<div align="center">
  <img src="./openflare_server/web/public/logo.png" width="120" height="120" alt="OpenFlare logo">

# OpenFlare

A lightweight, self-hosted OpenResty control plane for reverse proxy management, configuration rollout, node sync, TLS assets, and practical observability.

</div>

<p align="center">
  <a href="https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/LICENSE">
    <img src="https://img.shields.io/github/license/Rain-kl/OpenFlare?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/releases/latest">
    <img src="https://img.shields.io/github/v/release/Rain-kl/OpenFlare?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/pkgs/container/openflare">
    <img src="https://img.shields.io/badge/GHCR-ghcr.io%2Frain--kl%2Fopenflare-brightgreen" alt="ghcr">
  </a>
  <a href="https://goreportcard.com/report/github.com/Rain-kl/OpenFlare">
    <img src="https://goreportcard.com/badge/github.com/Rain-kl/OpenFlare" alt="GoReportCard">
  </a>
</p>

OpenFlare `1.0.0` is the current stable baseline. Phase six is complete and fully shipped; the repository documentation now focuses on the living system rather than historical implementation notes.

## Why It Exists

OpenFlare is built for a simple but recurring operational need:

* manage domain-to-origin reverse proxy rules from one control plane
* publish immutable OpenResty configuration versions
* let Agents pull, validate, reload, and roll back safely
* manage certificates, domains, node credentials, and version state
* expose practical dashboards for traffic, node health, and rollout status

It is not trying to be a CDN SaaS platform, a multi-tenant control plane, or a general-purpose logging system.

## Core Capabilities

* Versioned configuration with preview, publish, activate, and rollback
* Node onboarding with `discovery_token` or per-node `agent_token`
* Automated Agent apply flow with `openresty -t`, reload, and rollback
* Managed OpenResty templates, performance settings, and cache settings
* TLS certificate and domain management with exact and wildcard matching
* Request analytics, node snapshots, and health event reporting
* Controlled Server and Agent upgrade flows
* A production frontend built with Next.js App Router, React 19, and Tailwind CSS 4

## Architecture

```text
OpenFlare Server (Gin + GORM + SQLite/PostgreSQL + Web UI)
        |
        | HTTP API / Config Pull
        v
OpenFlare Agent (register / heartbeat / sync / apply / update)
        |
        v
Local OpenResty or Docker OpenResty
        |
        v
Origin
```

Responsibilities:

* `openflare_server`: admin UI, management APIs, Agent APIs, rendering, rollout, and state storage
* `openflare_agent`: node registration, heartbeat, sync, local apply, validation, reload, rollback, and self-update
* `openflare_server/web`: the production admin frontend, exported statically and served by the Go server

## UI Preview

### Dashboard Overview

![OpenFlare dashboard overview](./docs/assets/readme/dashboard-overview.png)

### Node Detail and Install Command

![OpenFlare node detail](./docs/assets/readme/node-detail.png)

### Version Release Workflow

![OpenFlare version release](./docs/assets/readme/version-release.png)

## Quick Start

### 1. Start the Server

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
      PORT: "3000"
    volumes:
      - openflare-data:/data

volumes:
  postgres-data:
  openflare-data:
```

```bash
docker compose up -d
```

Open `http://localhost:3000`

Default credentials:

* Username: `root`
* Password: `123456`

### 2. Install an Agent

First-time registration with `discovery_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

Registration with a per-node `agent_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

The installer writes to `/opt/openflare-agent` by default, creates `openflare-agent.service`, and can be re-run for reinstall or upgrade.

### 3. Publish Your First Config

1. Sign in and create a reverse proxy rule
2. Review the preview or diff
3. Activate the new version
4. Wait for Agents to pick it up on the next heartbeat

Version numbers follow `YYYYMMDD-NNN`. Versions are immutable; rollback is implemented by reactivating an older version.

## Repository Layout

* `openflare_server`: monolithic control plane built with Gin, GORM, and SQLite/PostgreSQL
* `openflare_server/web`: admin frontend built with Next.js 15 App Router
* `openflare_agent`: Go Agent
* `scripts`: install and helper scripts
* `docs`: design, guidelines, deployment, and configuration docs

## Local Development

### Server

```bash
cd openflare_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./openflare.db'
# Optional: switch to PostgreSQL by setting either DSN or SQL_DSN.
# If the PostgreSQL database is empty and ./openflare.db exists,
# OpenFlare migrates SQLite data automatically at startup.
# export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
go run .
```

### Frontend

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

### Agent

```bash
cd openflare_agent
go run ./cmd/agent -config /path/to/agent.json
```

### Useful Checks

```bash
cd openflare_server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

```bash
cd openflare_agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

## Admin Surface

The admin UI currently covers:

* reverse proxy rules
* config versions
* node management
* apply logs
* TLS certificates
* domain management
* user management
* settings
* version upgrades

Swagger UI is available at `/swagger/index.html` after login.

## License

OpenFlare is released under the [Apache License 2.0](./LICENSE).
