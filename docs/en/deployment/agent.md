# Access Agent

You will learn: The responsibilities of the Agent, the difference between the two access Tokens, installation script parameters, `agent.json` settings, and how to verify that the node has successfully connected.

The OpenFlare Agent runs on the proxy node. It does not receive arbitrary remote shell commands; instead, it pulls the configuration version published by the control plane via the Agent API, writes files for OpenResty locally, executes configuration validation, reloads, and attempts to roll back to a working configuration if it fails.

## Connection Credentials

| Method | Applicable Scenario |
| --- | --- |
| `discovery_token` | Automatically registers a node for the first time, which the Server exchanges for a node-specific credential |
| `agent_token` | Node has already been created/allocated in the management console, directly uses this node-specific credential |

At least one of `agent_token` or `discovery_token` must be configured.

### Credential Retrieval Path

- **`discovery_token` (Auto Registration Token)**: Log into the management console, navigate to "System Settings" -> "Auto Registration", where you can generate, view, and copy the global auto-registration credential.
- **`agent_token` (Node Specific Token)**: Log into the management console, navigate to "Node Management" -> "Add Node", fill in basic node information, save, and copy the node-specific access Token in the node details.

## One-Click Installation

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

The installation script downloads the latest Agent, writes to `/opt/openflare-agent` by default, generates `agent.json`, and registers `openflare-agent.service` on Linux + systemd environments.

Supported arguments:

| Argument | Description | Default Value |
| --- | --- | --- |
| `--server-url` | Server address (required) | |
| `--discovery-token` | One-time auto-registration Token | |
| `--agent-token` | Node-specific Token | |
| `--install-dir` | Target installation directory | `/opt/openflare-agent` |
| `--openresty-path` | Path to the OpenResty binary; automatically detects `openresty` if unspecified | |
| `--repo` | GitHub repository to download from | `Rain-kl/OpenFlare` |
| `--no-service` | Do not register systemd service | |

## Configuration File

Default configuration file path:

```text
/opt/openflare-agent/agent.json
```

Example local configuration:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_path": "openresty",
  "openresty_observability_port": 18081,
  "observability_replay_minutes": 15,
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

Example customized OpenResty paths configuration:

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
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

If `openresty_path` is not configured, the Agent calls `openresty` by default. For the full fields, see [Configurations Reference](../reference/configuration.md#agent-configurations-fields).

## Running in Docker

For Docker deployments, run the Agent image containing built-in OpenResty directly:

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker rm -f openflare-agent 2>/dev/null || true
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

## Start & Validate

In a systemd environment:

```bash
systemctl start openflare-agent
systemctl status openflare-agent
journalctl -u openflare-agent -f
```

Manual execution:

```bash
/opt/openflare-agent/openflare-agent -config /opt/openflare-agent/agent.json
```

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

Confirm in the management console:

| Position | Expected Result |
| --- | --- |
| Node List | Node status is online |
| Node Details | Heartbeat, current version, and basic resource metrics display correctly |
| Apply Logs | Application result displays after publishing |

## Uninstall

To completely uninstall the Agent and wipe local data:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

Supported arguments:

| Argument | Description | Default Value |
| --- | --- | --- |
| `--install-dir` | Installation directory | `/opt/openflare-agent` |
| `--service-name` | systemd service name | `openflare-agent` |

The uninstallation script only removes the Agent service, processes, and installation directory; it does not uninstall OpenResty from the host.

## Common Questions

| Symptom | Actions |
| --- | --- |
| `agent_token and discovery_token cannot both be empty` | Check if at least one Token is configured in `agent.json` |
| Node stays offline | Run `curl -I http://your-server:3000` on the Agent node to verify that the Server is reachable |
| OpenResty is not running | Review `journalctl -u openflare-agent`, checking that `openresty_path` is executable and ports 80/443 are not bound |
| Repeated application failures after publishing | The Agent blocks repeated sync attempts of the same failing `version + checksum`; fix the configuration and republish, or activate an older version to roll back |
