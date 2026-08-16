# Deployment Guide

You will learn: The recommended deployment strategies for OpenFlare, the system requirements for Server and Agent, how to run from source, integration steps, upgrades, and uninstallation entrypoints.

In production environments, we highly recommend using PostgreSQL as the Server database and explicitly configuring `SESSION_SECRET` for the Server. The recommended Agent deployment method is Docker (which runs the Agent image containing built-in OpenResty); host systemd service installation via script and manual local run are also supported.

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
OpenResty (Agent, WAF/HTTPS Termination)    <-- TunnelRelay Node
  |
  | proxy_pass (127.0.0.1:{vhost_port})
  v
OpenFlareRelay (frps process)              <-- TunnelRelay Node
  |
  | frp tunnel protocol
  v
OpenFlared (frpc client)                   <-- Intranet Server
  |
  v
Internal Service (192.168.x.x)
```

## Prerequisites

Server:

| Item | Requirement |
| --- | --- |
| Go | `1.25+`, required only when running from source |
| Node.js | `18+`, required only when building the admin frontend from source |
| Database | Writable SQLite parent directory, or a reachable PostgreSQL instance |
| Port | Listens on port `3000` by default |

Agent:

| Item | Requirement |
| --- | --- |
| System | The installation script supports Linux and macOS; the systemd service is created only on Linux + systemd environments |
| Architecture | `amd64` or `arm64` |
| OpenResty | Required to have the `openresty` executable when deploying locally, or specify its path via `--openresty-path` |
| Docker | Required only when deploying the Agent via Docker image |
| Network | The Agent node must be able to reach the Server address |
| GeoIP | WAF regional rules rely on the Agent's local MaxMind mmdb; the Agent initializes a built-in library on startup and updates it periodically |

### Hardware Allocation Recommendations

| Component | Minimum Allocation | Recommended Allocation | Note |
| --- | --- | --- | --- |
| **Server Control Plane** | 1 Core CPU / 1 GB RAM / 10 GB Disk | 2 Cores CPU / 4 GB RAM / 50 GB+ Disk | Expand disk allocation according to log retention windows and concurrency. |
| **Agent Data Plane** | 1 Core CPU / 512 MB RAM / 2 GB Disk | 2 Cores CPU / 2 GB RAM / 10 GB+ Disk | Expand according to concurrent reverse proxy connections and WAF workloads. |
| **Relay Node** | 1 Core CPU / 1 GB RAM / 5 GB Disk | 2 Cores CPU / 2 GB RAM / 20 GB Disk | frps throughput is primarily bounded by CPU processing capacity and bandwidth. |
| **OpenFlared Client** | 1 Core CPU / 256 MB RAM / 1 GB Disk | 1 Core CPU / 512 MB RAM / 5 GB Disk | Runs inside the intranet; utilizes minimal CPU/RAM, optimize for network throughput. |

## Docker Compose Deployment for Server

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
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openflare -d openflare"]
      interval: 10s
      timeout: 5s
      retries: 5

  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    container_name: openflare
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-a-long-random-string
      DSN: postgres://openflare:replace-with-strong-password@postgres:5432/openflare?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info
    volumes:
      - openflare-data:/data

volumes:
  postgres-data:
  openflare-data:
```

Start the Server:

```bash
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

Access `http://localhost:3000` for the first time, using the default credentials `root` / `123456`. Please change the default password immediately after logging in.

## Start Server from Source

First, build the admin frontend:

```bash
cd openflare-server/web
corepack enable
pnpm install
pnpm build
```

Then, launch the Server:

```bash
cd openflare-server
export SESSION_SECRET='replace-with-a-long-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
# Optional: Prefer PostgreSQL by setting DSN
# export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
go run .
```

By default, the Server listens on port `3000`. You can also specify it explicitly:

```bash
go run . --port 3000 --log-dir ./logs
```

## Running Agent in Docker (Recommended)

Docker is the recommended deployment method for the Agent. Running the Agent image directly launches the Agent controller alongside the built-in OpenResty binary. If `node_ip` is left blank, the Agent automatically resolves its outbound public IP via third-party APIs, avoiding registering the Docker bridge address as the node IP.

Mounting the configuration file:

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -v openflare-agent-data:/data \
  -v ./agent.json:/etc/openflare/agent.json:ro \
  ghcr.io/rain-kl/openflare-agent:latest
```

Using environment variables:

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

## Agent Connection via Installation Script

Apart from Docker, you can deploy the Agent directly on a Linux/macOS host using the installation script.

Auto-register using `discovery_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

Connect using node-specific `agent_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Installation script arguments:

| Argument | Description | Default Value |
| --- | --- | --- |
| `--server-url` | Server address (required) | |
| `--discovery-token` | Auto-registration Token; mutually exclusive with `--agent-token` | |
| `--agent-token` | Node-specific Token; mutually exclusive with `--discovery-token` | |
| `--install-dir` | Target installation directory | `/opt/openflare-agent` |
| `--openresty-path` | Path to the OpenResty binary; automatically detects `openresty` if unspecified | |
| `--repo` | GitHub repository to download from | `Rain-kl/OpenFlare` |
| `--no-service` | Do not register systemd service | |

Confirm service status:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

## Running the Agent Manually

Running from source:

```bash
cd openflare-agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

Running compiled binary:

```bash
cd openflare-agent
go build -o openflare-agent ./cmd/agent
export LOG_LEVEL='info'
./openflare-agent -config /path/to/agent.json
```

Minimal `agent.json` example:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_path": "openresty",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

If `openresty_path` is left blank, the Agent calls `openresty` by default.

By default, the Agent attempts to upgrade the HTTP heartbeat connection to WebSocket once successfully registered. Once upgraded, configuration activations on the Server notify the Agent instantly; if WebSocket disconnects or fails to establish, the Agent gracefully falls back to HTTP polling.

WAF geographical filtering depends on the local `GeoLite2-Country.mmdb`. The Agent automatically writes the built-in database to `data_dir/etc/openflare/GeoLite2-Country.mmdb` on startup and checks for periodic updates. Muted warnings are logged if updates fail, having no impact on Nginx configuration sync or reloads.

## Upgrades & Uninstallation

Server:

* Root users can check and trigger Server upgrades in the top header of the management console.
* To deploy preview releases, manually check the GitHub Releases page.
* You can also trigger upgrades by uploading the compiled Server binary in the console.

Agent:

* By default, the Agent automatically upgrades following stable releases.
* Agent self-updates require the GitHub Release to contain the compiled binary and a matching `.sha256` checksum file; updates are blocked if the downloaded binary fails the SHA-256 validation.
* You can re-execute the installation script to redeploy or force-update the Agent.
* Upgrading to preview releases requires a manual trigger.

Uninstalling the Agent:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

The uninstallation script stops the Agent process, removes the systemd service unit, and wipes the installation directory, without uninstalling OpenResty from the host.
