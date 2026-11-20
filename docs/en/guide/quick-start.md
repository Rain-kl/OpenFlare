# Quick Start

You will learn: how to start the OpenFlare Server with Docker Compose, complete the first login, connect your first Agent, and verify that a config has been published to a node.

OpenFlare's minimal runtime consists of:

| Component | Responsibility |
| --- | --- |
| Server | admin UI, admin API, Agent API, config rendering, version release, and state storage |
| Pingora Edge | Runs on edge hosts, synchronizes configuration, and proxies requests to origin servers |

## Environment Requirements

| Item | Requirement |
| --- | --- |
| Docker / Docker Compose | starts the Server and its PostgreSQL, Valkey dependencies; also runs the Agent if using the Docker Agent |
| Reachable port | Server listens on `3000` by default; Agent nodes must be able to reach the Server address |

---

## 1. Start the Server

Quick start recommends the standard **PostgreSQL + Valkey** deployment.

Create a `docker-compose.yaml` in an empty directory:

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
      - openflare_uploads:/app/uploads
    environment:
      TZ: Asia/Shanghai
      APP_SESSION_SECRET: 'replace-with-a-long-random-string' # replace with a long random string in production
      DB_ENABLED: "true"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USERNAME: "${DB_USERNAME:-openflare}"
      DB_PASSWORD: "${DB_PASSWORD:-replace-with-strong-password}"
      DB_NAME: "${DB_NAME:-openflare}"
      REDIS_ENABLED: "true"
      REDIS_ADDR: "redis:6379"
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


volumes:
    openflare_uploads:
    openflare_postgres_data:
    openflare_redis_data:
```

Start the services:

```bash
docker compose up -d
```

Confirm the containers are running:

```bash
docker compose ps
docker compose logs -f openflare
```

After seeing `server listening` and the `openflare-server` container status running, open in a browser:

```text
http://localhost:3000
```

Default account:

| Username | Password |
| --- | --- |
| `admin` | `12345678` |

> [!WARNING]
> For your system's security, change the default password immediately after the first login.

If you forget the password and no password-recovery channel is configured, reset it with:

```bash
go run ./cmd/server reset-passwd --user admin
```

Without `--password`, the command auto-generates a random password and prints it to the terminal; you can also explicitly specify a new password with `--password`.

---

## 2. Prepare an Agent Token

Agents can connect with two credential types:

| Credential | Use Case |
| --- | --- |
| `discovery_token` | first-time auto registration; the Server exchanges it for a node-specific Token |
| `agent_token` | node already created or assigned in the admin panel; use the node-specific Token directly |

Prepare one of these credentials in the admin panel, then continue.

- **`discovery_token`** menu path:「System Settings」->「OpenFlare」tab ->「Discovery Token & Deployment」→ Discovery Token
- **`agent_token`** menu path: after creating a node in「Node Management」, click into the node detail page to see its dedicated Token.

---

## 3. Install Pingora Edge

Run the installer on a Linux edge host. Use the discovery token for a new node, or copy the command with a node-specific token from the node details.

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type discovery \
  --token '<discovery token>'
```

The installer creates the `openflare-edge` systemd service. It listens on TCP ports 80 and 443 by default and waits for a published site snapshot to become active. Assign a policy to the node in **Performance policies**.

## 4. Next Steps

After starting the control panel and connecting an Agent node, you've successfully built the base runtime environment of the OpenFlare gateway. Continue with these two guides to deploy your first reverse proxy site:

1. **Publish your first website**:
   * See [Create Your First Reverse Proxy Rule](./first-site.md). It guides you to create your first proxy rule over HTTP and verify the node applied it.
2. **Full reverse proxy config (HTTPS & origin management)**:
   * See [Create a Reverse Proxy Config](./proxy-config.md). It guides you from certificate import/application to domain HTTPS certificate binding, origin management, and preview release.

---

## When You Hit Problems

Handle in this order:

1. Upgrade Server and Agent to the latest version; confirm whether the problem persists.
2. Check the node’s apply records and save any configuration corrections.
3. Run「Force Sync」on the target node in the node detail page to push an immediate config pull.
4. Rebuild or reinstall the Agent (re-run the install script).
5. If none of the above works, file a [GitHub Issue](https://github.com/Rain-kl/OpenFlare/issues) with the Server logs and node apply records.

More troubleshooting: [Troubleshooting](./troubleshooting.md).

---

## Advanced Deployment Guides

After completing the quick start and getting familiar with OpenFlare, read these advanced deployment docs to put components into production:

* **Server production deployment**: read [Start the Server](../deployment/server.md) for building the frontend from source, system env vars, and Docker Compose.
* **Agent production access**: read [Access Agent](../deployment/agent.md) for systemd service management, detailed local config file fields, and troubleshooting.
* **Intranet relay deployment**: read [Deploy Relay](../deployment/relay.md) for configuring public relay nodes (frps) for tunnels.
* **Intranet client deployment**: read [Deploy OpenFlared](../deployment/openflared.md) for running the tunnel daemon client (frpc) on the intranet server.
* **Production topology reference**: read [Deployment Guide](../deployment/deployment.md) for production HA topology and overall network planning.
* **Upgrades and maintenance**: read [Upgrade & Maintenance](../deployment/upgrade.md) for smooth upgrades of the Server and Agent nodes.
