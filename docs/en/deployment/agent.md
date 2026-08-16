# Access Agent

You will learn: the Agent's responsibilities, the difference between the two access Tokens, install script parameters, `agent.json` config, and how to confirm the node is online.

The OpenFlare Agent runs on the proxy node side. It doesn't accept remote shell commands; instead it pulls released config versions from the control plane via the Agent API, writes OpenResty files locally, runs config validation, reloads, and attempts to roll back to a runnable config on failure.

## Connection Methods

| Method | Use Case |
| --- | --- |
| `discovery_token` | first-time auto-registration; the Server exchanges it for a node-specific credential |
| `agent_token` | node already created/assigned in the admin panel; connect with the node-specific credential |

At least one of `agent_token` / `discovery_token` is required.

### Credential Paths

- **`discovery_token` (auto-registration credential)**: log in to the admin panel, navigate to「System Settings」->「Auto Registration」; generate, view, and copy the global auto-registration credential there.
- **`agent_token` (node-specific credential)**: log in to the admin panel, navigate to「Node Management」->「Add Node」; after filling in basic info and saving, copy the node-specific access Token on the node detail page.

## One-Click Install

### Interactive Install (recommended)

Running the install script without any arguments enters interactive mode, with a wizard choosing the install method (local / Docker container) and configuring the Server address and auth Token (if Docker is chosen and Docker isn't installed locally, the script asks and intelligently installs Docker):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash
```

### Automated (non-interactive) Install

Adding any arguments enters automated install mode with no interaction.

Local install with `discovery_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

Local install with node-specific `agent_token`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Automated Docker container install:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN \
  --docker
```

In local install mode, the script downloads the latest Agent, writes to `/opt/openflare-agent` by default, generates `agent.json`, auto-detects and creates the low-privilege system account `openflare` (granting the whole install dir to it), and creates the `openflare-agent.service` systemd service on Linux + systemd. The service runs as the `openflare` unprivileged user, with Linux Capabilities (`CAP_NET_BIND_SERVICE`) enabling privileged ports (e.g. 80, 443).

Supported parameters:

| Parameter | Description |
| --- | --- |
| `--server-url` | Server address |
| `--discovery-token` | first-time auto-registration Token |
| `--agent-token` | node-specific Token |
| `--install-dir` | install dir, default `/opt/openflare-agent` (local install only) |
| `--openresty-path` | OpenResty binary path; auto-finds `openresty` when omitted (local install only) |
| `--repo` | GitHub repo for downloading the Agent, default `Rain-kl/OpenFlare` |
| `--no-service` | don't create the systemd service (local install only) |
| `--docker` | install via Docker container |
| `--method` | install method: `local` or `docker` (default `local`) |

## Config File

Default config file path:

```text
/opt/openflare-agent/agent.json
```

Local config example:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_path": "openresty",
  "openresty_observability_port": 18081,
  "observability_replay_minutes": 60,
  "heartbeat_interval": 3000,
  "request_timeout": 10000
}
```

Custom OpenResty path example:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "/var/lib/openflare-agent",
  "openresty_path": "/usr/local/openresty/nginx/sbin/openresty",
  "main_config_path": "/var/lib/openflare-agent/etc/nginx/nginx.conf",
  "route_config_path": "/var/lib/openflare-agent/etc/nginx/conf.d/openflare_routes.conf",
  "access_log_path": "/var/lib/openflare-agent/var/log/openflare/access.log",
  "cert_dir": "/var/lib/openflare-agent/etc/nginx/certs",
  "lua_dir": "/var/lib/openflare-agent/etc/nginx/lua",
  "runtime_config_dir": "/var/lib/openflare-agent/etc/openflare",
  "heartbeat_interval": 3000,
  "request_timeout": 10000
}
```

Without `openresty_path`, the Agent calls `openresty` by default. Full fields: [Configuration Reference](../reference/configuration.md#agent-命令行参数与配置字段).

## Running with Docker

For Docker deployment, directly run the Agent image with a built-in OpenResty:

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

> [!NOTE]
> **Pages persistence**
> By default the Pages deployment dir is mounted to the Docker named volume `openflare-agent-pages` (container path `/data/var/lib/openflare/pages`). Rebuilding or upgrading the Agent container doesn't require re-pulling static site packages.

## Uninstall

### Interactive Uninstall (recommended)

Running the uninstall script without any arguments enters interactive mode with a menu choosing the method (local uninstall / Docker container uninstall):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

### Docker Container Uninstall

Stop and remove the `openflare-agent` container.

## FAQ

| Symptom | Handling |
| --- | --- |
| `agent_token and discovery_token cannot both be empty` | check that `agent.json` has at least one Token |
| Node stays offline | run `curl -I http://your-server:3000` on the Agent node to confirm the Server address is reachable |
| Repeated failure after release | the Agent blocks re-applying the same `version + checksum`; click「Force Sync」in the node detail page, or republish a new version |
