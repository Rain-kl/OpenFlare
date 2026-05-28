# Quick Start

You will learn how to start OpenFlare Server with Docker Compose, sign in for the first time, connect the first Agent, and verify that a configuration was published to a node.

The minimal OpenFlare setup contains:

| Component | Responsibility |
| --- | --- |
| Server | Management UI, management API, Agent API, configuration rendering, release publishing, and state storage |
| Agent | Runs on proxy nodes, pulls configuration, writes OpenResty files, validates, and reloads |
| OpenResty | Receives traffic and proxies requests to origins |

Agent controls OpenResty through the OpenResty binary. Local installs need an `openresty` executable on the node; Docker installs can run the Agent image that already includes OpenResty.

## Requirements

| Item | Requirement |
| --- | --- |
| Docker / Docker Compose | Used to start Server and PostgreSQL; also used if you run the Agent Docker image |
| OpenResty | Required for local Agent installs unless `--openresty-path` points to a custom binary |
| Reachable ports | Server listens on `3000` by default. Agent nodes must reach the Server URL. |
| Browser | Used to open the management UI |

[Needs confirmation: minimum recommended Docker and Docker Compose versions]

## 1. Start Server

Create `docker-compose.yml` in an empty directory:

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
  postgres-data:
```

Start:

```bash
docker compose up -d
```

Verify:

```bash
docker compose ps
docker compose logs -f openflare
```

When the `openflare` container is running and logs show `server listening`, open:

```text
http://localhost:3000
```

Default account:

| Username | Password |
| --- | --- |
| `root` | `123456` |

Change the default password immediately after first login.

## 2. Prepare an Agent Token

Agents can connect with either:

| Credential | Use Case |
| --- | --- |
| `discovery_token` | First-time automatic node registration. Server exchanges it for a node-specific token. |
| `agent_token` | A node-specific token created or assigned in the management UI. |

Prepare one of them in the management UI before continuing.

[Needs confirmation: exact UI menu path for creating or viewing `discovery_token` and node `agent_token`]

## 3. Install Agent

Run the install script on the proxy node.

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

The script defaults to:

| Item | Default |
| --- | --- |
| Install directory | `/opt/openflare-agent` |
| Config file | `/opt/openflare-agent/agent.json` |
| systemd service | `openflare-agent.service` |
| OpenResty path | Auto-detects `openresty` unless `--openresty-path` is provided |

Check status:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

If systemd is unavailable, the script prints a manual start command.

## 4. Publish the First Configuration

In the management UI:

1. Create a site configuration with a site name, domain, and origin URL.
2. Ensure the site is enabled.
3. Preview the rendered configuration or review the diff.
4. Publish and activate a new version.
5. Wait for the Agent to discover and apply the version through heartbeat.

Version numbers use `YYYYMMDD-NNN`. Historical versions are immutable; rollback reactivates an old version.

## 5. Verify Success

In the UI:

| Location | Expected Result |
| --- | --- |
| Node list | Agent node is online |
| Node detail | Current version matches the active version |
| Apply logs | Latest apply succeeded |
| Versions page | New version is active |

On the Agent node:

```bash
journalctl -u openflare-agent -n 100 --no-pager
```

## Common Failures

| Symptom | What to Check |
| --- | --- |
| Cannot open the UI | Confirm `docker compose ps` shows Server running and host port `3000` is free |
| Login works but data cannot be saved | Check PostgreSQL health and the username/password/database in `DSN` |
| Agent cannot register | Confirm the Agent node can reach `--server-url`, and check whether the token is wrong or expired |
| Agent is online but does not apply | Confirm the site is enabled and a version was published and activated |
| OpenResty apply fails | Check apply logs and `journalctl -u openflare-agent`, especially domains, certificates, upstream URLs, and port conflicts |

See [Troubleshooting](./troubleshooting.md) for deeper diagnostics.
