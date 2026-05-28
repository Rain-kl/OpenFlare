# Deployment

You will learn the recommended OpenFlare deployment model, Server and Agent requirements, source startup workflow, integration steps, upgrade paths, and uninstall entry points.

For production, use PostgreSQL for the Server database and set `SESSION_SECRET` explicitly. Agent controls OpenResty through the OpenResty binary; Docker deployments run the Agent image that already includes OpenResty.

## Topology

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

## Requirements

Server:

| Item | Requirement |
| --- | --- |
| Go | `1.25+`, source run only |
| Node.js | `18+`, frontend source build only |
| Database | Writable SQLite directory or reachable PostgreSQL instance |
| Port | `3000` by default |

Agent:

| Item | Requirement |
| --- | --- |
| OS | Install script supports Linux and macOS. systemd service is created only on Linux + systemd. |
| Architecture | `amd64` or `arm64` |
| OpenResty | Required for local Agent installs |
| Docker | Required only when running the Agent Docker image |
| Network | Agent node must reach the Server URL |

[Needs confirmation: recommended production CPU, memory, and disk size]

## Docker Compose Server

Create `docker-compose.yml`:

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

Start:

```bash
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

Open `http://localhost:3000`. The default account is `root` / `123456`; change it immediately.

## Run Server from Source

Build the management UI first:

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

Then start Server:

```bash
cd openflare_server
export SESSION_SECRET='replace-with-a-long-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
# Optional: PostgreSQL takes precedence when set.
# export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
go run .
```

Default port is `3000`. You can also set it explicitly:

```bash
go run . --port 3000 --log-dir ./logs
```

## Connect Agent

With `discovery_token`:

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

Supported options:

| Option | Description |
| --- | --- |
| `--server-url` | Server URL, required |
| `--discovery-token` | First-registration token, mutually exclusive with `--agent-token` |
| `--agent-token` | Node-specific token, mutually exclusive with `--discovery-token` |
| `--install-dir` | Install directory, default `/opt/openflare-agent` |
| `--openresty-path` | OpenResty binary path, auto-detected when omitted |
| `--repo` | GitHub repository for Agent downloads, default `Rain-kl/OpenFlare` |
| `--no-service` | Do not create a systemd service |

Check status:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

## Run Agent Manually

From source:

```bash
cd openflare_agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

Build and run:

```bash
cd openflare_agent
go build -o openflare-agent ./cmd/agent
export LOG_LEVEL='info'
./openflare-agent -config /path/to/agent.json
```

Minimal `agent.json`:

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

When `openresty_path` is not configured, Agent runs `openresty`.

## Minimal Integration Flow

1. Start Server and sign in.
2. Prepare `agent_token` or `discovery_token`.
3. Start Agent and confirm the node is online.
4. Create an enabled site configuration.
5. Publish and activate a new version.
6. Check node detail and apply logs.
7. Visit the domain or verify with `curl`.

## Upgrade and Uninstall

Server:

* Root users can check and upgrade stable Server releases from the top bar.
* Preview releases can be checked manually.
* Binary upload upgrades are also supported.

Agent:

* Agents follow stable releases by default.
* The install script can be rerun to reinstall or upgrade.
* Preview upgrades require manual action.

Uninstall Agent:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

The uninstall script stops Agent and removes the systemd service and install directory. It does not remove the local OpenResty installation.

## Validation Commands

Server:

```bash
cd openflare_server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Agent:

```bash
cd openflare_agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Frontend:

```bash
cd openflare_server/web
pnpm build
```

Swagger:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd openflare_server
swag init -g main.go -o docs
```
