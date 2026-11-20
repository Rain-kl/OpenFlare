# Deployment Guide

You will learn: OpenFlare's recommended deployment approaches, Server and Agent runtime requirements, source startup, integration steps, and upgrade/uninstall entries.

Production recommends PostgreSQL as the Server DB, with `APP_SESSION_SECRET` etc. configured via `config.yaml` or env vars. The full Docker Compose deployment requires Redis; ClickHouse is optional for massive access logs and observability time series (see the repo root `docker-compose.yaml`). The Agent supports both Docker deployment and a local install script; the Docker image bundles the Pingora binary. Log-DB determination and switching: [Log Store Decoupling](../design/logstore.md).

## Deployment Topology

### Standard Reverse Proxy Traffic Path

```text
Browser
  |
  v
OpenFlare Server :3000
  |
  | Agent API / heartbeat / config pull
  v
OpenFlare Agent
  |
  v
Pingora binary
  |
  v
Origin service
```

### Intranet Penetration Traffic Path

```text
Browser
  |
  v
Pingora (Agent, WAF/HTTPS termination)      <-- TunnelRelay node
  |
  | proxy_pass (127.0.0.1:{vhost_port})
  v
OpenFlareRelay (frps process)                 <-- TunnelRelay node
  |
  | frp tunnel protocol
  v
OpenFlared (frpc client)                      <-- intranet server
  |
  v
Internal Service (192.168.x.x)
```

## Prerequisites

### Hardware Recommendations

| Component | Reference (entry) | Reference (production) | Notes |
| --- | --- | --- | --- |
| **Server control plane** | 1 core / 2 GB RAM / 20 GB disk | 2 cores / 4 GB RAM / 50 GB+ disk | expand disk by access-log retention and concurrent traffic |
| **Agent data plane** | 1 core / 512 MB RAM / 2 GB disk | 2 cores / 2 GB RAM / 10 GB+ disk | expand by Pingora concurrent proxy connections and WAF interception |
| **Relay node** | 1 core / 1 GB RAM / 5 GB disk | 2 cores / 2 GB RAM / 20 GB disk | frps relay throughput limited by bandwidth and CPU |
| **OpenFlared client** | 1 core / 256 MB RAM / 1 GB disk | 1 core / 512 MB RAM / 5 GB disk | runs independently in the intranet, tiny footprint |

## Docker Compose Server Deployment

The repo root provides a full `docker-compose.yaml` (PostgreSQL, Redis, ClickHouse, Jaeger).

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# edit .env; at minimum change APP_SESSION_SECRET and the DB passwords
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

First visit `http://localhost:3000`; default account `admin` / `12345678`. Change the default password immediately after login.

## Source Startup

First build the admin frontend:

```bash
cd frontend
corepack enable
pnpm install
pnpm build:embed
```

Then start the Server (repo root):

```bash
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
# optional: use PostgreSQL
# export DB_HOST=127.0.0.1 DB_USERNAME=postgres DB_PASSWORD=postgres DB_NAME=openflare
go run ./cmd/server all
```

Listens on `:3000` by default (controlled by `app.addr` in `config.yaml` or `APP_ADDR`).

## Deploy edge nodes

Follow [Deploy Pingora Edge](./agent.md). Publish a site snapshot, confirm that the node is online, and assign a performance policy in the console.
