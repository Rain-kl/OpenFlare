# Quick Start

You will learn: How to start OpenFlare Server using Docker Compose, complete your first login, connect your first Agent, and verify if a configuration has been published to the node.

The minimum running unit of OpenFlare consists of:

| Component | Responsibility |
| --- | --- |
| Server | Admin UI, Admin API, Agent API, configuration rendering, version publishing, and state storage. |
| Agent | Runs on the proxy node, pulls configurations, writes files for OpenResty, executes validations, and triggers reloads. |
| OpenResty | Receives actual traffic and reverse proxies it to origin servers. |

The Agent manages the runtime through the OpenResty binary. A local deployment requires the `openresty` executable to be already present on the node; a Docker deployment can directly run the Agent image containing built-in OpenResty.

## Environment Requirements

| Item | Requirement |
| --- | --- |
| Docker / Docker Compose | Used to start Server and PostgreSQL; also used to run the Agent if using the Docker Agent image |
| OpenResty | Required to have the `openresty` executable when installing the Agent locally, or specify its path in the installation script |
| Reachable Ports | The Server listens on port `3000` by default; the Agent node needs to be able to reach the Server address |
| Browser | Used to access the management console |

* **Docker**: `20.10.0+`
* **Docker Compose**: `2.0.0+`

## 1. Start the Server

Create a `docker-compose.yml` file in an empty directory:

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

Start the services:

```bash
docker compose up -d
```

Verify that the containers are running:

```bash
docker compose ps
docker compose logs -f openflare
```

Once you see `server listening` in the logs and the `openflare` container status is running, access:

```text
http://localhost:3000
```

Default credentials:

| Username | Password |
| --- | --- |
| `root` | `123456` |

Please change the default password immediately after your first login.

## 2. Prepare Agent Token

The Agent can be connected using one of two types of credentials:

| Credential | Applicable Scenario |
| --- | --- |
| `discovery_token` | Automatically registers a node for the first time, which the Server exchanges for a node-specific Token |
| `agent_token` | Node has already been created/allocated in the management console, directly uses this node-specific Token |

After preparing one of these credentials in the management console, proceed to the next step.

* **`discovery_token`** path: "System Settings" -> "Auto Registration"
* **`agent_token`** path: "Node Management" -> "Add Node"

## 3. Install/Run the Agent

The recommended Agent deployment method is using Docker (which runs the Agent image with built-in OpenResty); deploying the Agent locally on the host using the installation script is also supported.

### Option A: Run Agent in Docker (Recommended)

Run the Agent image directly on the proxy node:

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -v openflare-agent-data:/data \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

### Option B: Execute Installation Script (Local Host Deployment)

Execute the installation script on the proxy node.

Using the `discovery_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

Using the node-specific `agent_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

The script defaults to:

| Item | Default Value |
| --- | --- |
| Install Directory | `/opt/openflare-agent` |
| Config File | `/opt/openflare-agent/agent.json` |
| systemd Service | `openflare-agent.service` |
| OpenResty Path | Automatically detects `openresty` if unspecified |

Verify the Agent service status:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

If systemd is not available on the OS, the script outputs manual startup commands instead.

## 4. Publish Your First Configuration

Perform the following operations in the management console:

1. Add a website configuration, filling in the website name, domain, and origin address.
2. Verify that the website configuration is enabled.
3. Check the preview or change summary before publishing.
4. Publish and activate the new version.
5. Wait for the Agent to detect and apply the version in the next heartbeat.

The version number format is `YYYYMMDD-NNN`. Historic versions are immutable; rollbacks are accomplished by re-activating an older version.

## 5. Verify Success

Confirm in the management console:

| Position | Expected Result |
| --- | --- |
| Node List | Agent node status is online |
| Node Details | Current version matches active version |
| Apply Logs | Most recent application succeeded |
| Version Page | The new version is currently active |

Confirm on the Agent node:

```bash
journalctl -u openflare-agent -n 100 --no-pager
```

## Common Failures

| Symptom | Troubleshooting Direction |
| --- | --- |
| Management console fails to load in browser | Verify that the Server is running in `docker compose ps` and port `3000` is not bound by other processes |
| Data fails to save after logging in | Check the health of the PostgreSQL container, and verify the username, password, and database name in `DSN` |
| Agent fails to register | Verify that the Agent node can reach `--server-url`, and verify if the Token is typed correctly or expired |
| Agent is online but configuration is not applied | Verify that the website configuration is enabled and a version has been published and activated |
| OpenResty application fails | Review node application logs and `journalctl -u openflare-agent`, checking domains, certificates, upstreams, and port conflicts |

For more troubleshooting details, see [Troubleshooting](./troubleshooting.md).

---

## Advanced Deployment Guides

Once you complete the quick start and familiarize yourself with the basic operations of OpenFlare, you can read the following advanced deployment documents to put components into production:

* **Server Production Deployment**: Read [Launch Server](../deployment/server.md) to learn how to build the frontend from source, configure system environment variables, and run with Docker Compose.
* **Agent Production Integration**: Read [Deploy Agent](../deployment/agent.md) to learn about systemd-based service management, detailed local configuration parameters, and troubleshooting.
* **Tunnel Relay Deployment**: Read [Deploy Relay](../deployment/relay.md) to learn how to configure public relay nodes (frps) for penetration tunnels.
* **Tunnel Client Deployment**: Read [Deploy OpenFlared](../deployment/openflared.md) to learn how to run the penetration daemon client (frpc) on the intranet server side.
* **Production Deployment Topology**: Read [Deployment Guide](../deployment/deployment.md) to learn about high-availability production topologies and overall network planning.
* **System Upgrades & Maintenance**: Read [Upgrade & Maintenance](../deployment/upgrade.md) to learn how to upgrade the Server and individual node Agents smoothly.
