# Deploy Relay (Tunnel relay)

TunnelRelay nodes serve as public relay endpoints for the OpenFlare intranet penetration infrastructure. Running the `openflare-relay` supervisor and the `frps` (Fast Reverse Proxy Server) daemon, the relay maintains public tunnel connections, receives proxied traffic from Pingora edge gateways, and dispatches it across encrypted tunnels to OpenFlared clients inside private networks.

---

## Prerequisites

Before deploying a TunnelRelay node, confirm:

1. Node registration: Create a node of type `tunnel_relay` in the OpenFlare admin console under Node Management, and retrieve its dedicated `agent_token` (or use the global `discovery_token`).
2. Network ports:
   - `7000` (TCP): frpc client connection port (`bindPort`), accessible from public client hosts.
   - `8080` (TCP): HTTP virtual host port (`vhostHTTPPort`), used by Pingora edge gateways to forward traffic to frps (typically local loopback or private subnet).
   - `17500` (TCP, optional): Embedded frps Web UI management port (if enabled in system settings).
3. Software dependency (bare-metal host deployment only): An executable `frps` binary available in the system path or explicitly specified via configuration.

---

## Configuration and environment variables

`openflare-relay` reads `relay.json` in the working directory by default. All settings can be overridden via environment variables:

| JSON field | Environment variable | Description | Default |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server control plane endpoint | **Required** |
| `agent_token` | `OPENFLARE_AGENT_TOKEN` | Dedicated node token | One of two required |
| `discovery_token` | `OPENFLARE_DISCOVERY_TOKEN` | Discovery enrollment token | One of two required |
| `node_name` | `OPENFLARE_NODE_NAME` | Node identifier | Host system hostname |
| `node_ip` | `OPENFLARE_NODE_IP` | Public egress/listen IP address | Auto-detected |
| `frps_path` | `OPENFLARE_FRPS_PATH` | Path to executable `frps` binary | `"frps"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | Working directory for generated `frps.toml` | `"./data"` |
| `state_path` | - | Path to local state file | `"{data_dir}/relay-state.json"` |
| `heartbeat_interval`| - | Heartbeat interval to Server (milliseconds or Go Duration) | `10000` (10s) |
| `request_timeout` | - | HTTP API request timeout (milliseconds or Go Duration) | `10000` (10s) |

---

## Docker container deployment

The official Docker image bundles the `openflare-relay` supervisor and the `frps` binary:

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7000:7000 \
  -p 17500:17500 \
  -e OPENFLARE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  -v openflare-relay-data:/app/data \
  ghcr.io/rain-kl/openflare-relay:latest
```

Notes:
- `-p 7000:7000` maps the primary communication port for connecting frpc clients. If `relay_bind_port` is customized in the Server settings, update the host port mapping accordingly.
- Embedded frps Web UI: If enabled in system settings (`relay_frps_web_ui_enabled: true`), map port `17500` to the host. The Web UI authenticates using username `admin` and the node's `agent_token` as the password.

---

## Runtime verification

### Automation workflow

Upon startup, the Relay performs the following tasks:
1. Emits an HTTP heartbeat to the Server to register and report online status.
2. Pulls base relay configuration (including `bindPort`, `vhostHTTPPort`, and the dynamically generated tunnel authorization token `auth_token`).
3. Renders the local `data/frps.toml` configuration file.
4. Spawns and supervises `frps -c data/frps.toml`.
5. If `frps` crashes, the supervisor restarts it with exponential backoff (1-second initial delay, capped at 60 seconds).

### View service logs

```bash
docker logs -f openflare-relay
```

### Admin console verification

Log in to the OpenFlare admin console and navigate to Node Management:
- Verify the TunnelRelay node is marked as online.
- Verify the node type is listed as Relay Node and the frps daemon status reports Healthy.
