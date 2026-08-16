# Deployment Guide

You will learn: OpenFlare's recommended deployment approaches, Server and Agent runtime requirements, source startup, integration steps, and upgrade/uninstall entries.

Production recommends PostgreSQL as the Server DB, with `APP_SESSION_SECRET` etc. configured via `config.yaml` or env vars. The full Docker Compose deployment requires Redis; ClickHouse is optional for massive access logs and observability time series (see the repo root `docker-compose.yaml`). The Agent supports both Docker deployment and a local install script; the Docker image bundles the OpenResty binary. Log-DB determination and switching: [Log Store Decoupling](../design/logstore.md).

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
OpenResty binary
  |
  v
Origin service
```

### Intranet Penetration Traffic Path

```text
Browser
  |
  v
OpenResty (Agent, WAF/HTTPS termination)      <-- TunnelRelay node
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
| **Agent data plane** | 1 core / 512 MB RAM / 2 GB disk | 2 cores / 2 GB RAM / 10 GB+ disk | expand by OpenResty concurrent proxy connections and WAF interception |
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
go run main.go all
```

Listens on `:3000` by default (controlled by `app.addr` in `config.yaml` or `APP_ADDR`).

## Run the Agent with Docker (recommended)

Docker is the recommended Agent deployment. The Agent image is built on the OpenResty image, bundling the Agent controller and the OpenResty binary. Without an explicit `node_ip`, the Agent prefers fetching the real egress IP via a third-party API, avoiding registering the Docker bridge address as the node IP.

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -v openflare-agent-pages:/data/var/lib/openflare/pages \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

The named volume `openflare-agent-pages` persists the Pages deployment dir; rebuilding the container doesn't require re-pulling static site packages.

## Agent Access (script install)

Besides Docker, the install script can deploy the Agent to the local host. The script registers the low-privilege `openflare` service account and runs the systemd service as that user, using Linux Capabilities to safely listen on privileged ports 80/443.

Auto-register with `discovery_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

With node-specific `agent_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Install script parameters:

| Parameter | Description |
| --- | --- |
| `--server-url` | Server address, required |
| `--discovery-token` | first-time auto-registration Token, one of two with `--agent-token` |
| `--agent-token` | node-specific Token, one of two with `--discovery-token` |
| `--install-dir` | install dir, default `/opt/openflare-agent` |
| `--openresty-path` | OpenResty binary path; auto-finds `openresty` when omitted |
| `--repo` | GitHub repo for downloading the Agent, default `Rain-kl/OpenFlare` |
| `--no-service` | don't create the systemd service |

Confirm state:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```
