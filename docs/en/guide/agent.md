# Connect Agent

OpenFlare Agent runs on proxy nodes. It handles registration, heartbeat, configuration sync, OpenResty file writes, validation, reload, rollback, and self-update.

## Authentication

| Method | Use case |
| --- | --- |
| `agent_token` | The node already exists or has a dedicated credential |
| `discovery_token` | First-time auto-registration; Server exchanges it for a node token |

At least one of them is required.

## Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Or with discovery:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

## Configuration Example

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

Without `openresty_path`, Agent runs `openresty` by default.

## Docker

```bash
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443 -p 127.0.0.1:18081:18081 \
  -v openflare-agent-data:/data \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  ghcr.io/rain-kl/openflare-agent:latest
```

## Run from Source

```bash
cd openflare_agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```
