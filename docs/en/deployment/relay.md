# Deploy Relay (Tunnel Relay)

You will learn: TunnelRelay node responsibilities, `openflare-relay` config items and env vars, running Relay with Docker, and building/deploying Relay manually from source.

In OpenFlare's intranet penetration system, the **TunnelRelay node** plays a key role. Unlike regular edge nodes, besides running the traditional Agent (hosting OpenResty for HTTPS/WAF processing), it also runs the **Relay (frps tunnel manager)** service on the same machine, listening for tunnel connections from intranet clients (OpenFlared) and relaying traffic.

---

## Prerequisites

Before deploying a TunnelRelay node, make sure:

1. **Registered as a TunnelRelay-type node**: in OpenFlare admin「Node Management」, add a node of type `tunnel_relay` and get its dedicated `agent_token`, or use the global `discovery_token`.
2. **Network ports**:
   - `bindPort` (frpc connection port, default `7000`) must be reachable by public/intranet clients.
   - `vhostHTTPPort` (HTTP Vhost port, default `8080`) must be free; the Agent exchanges traffic with frps on this port.
3. **Software dependency** (host deployment only):
   - an executable `frps` binary locally, or an explicitly specified path via parameter.

---

## Config File and Env Vars

`openflare-relay` reads `relay.json` in the current directory by default at startup, fully overridable via env vars.

### Config Field Details

| JSON field | Env var | Description | Default |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server API service address | **none (required)** |
| `agent_token` | `OPENFLARE_AGENT_TOKEN` | node-specific Token | one of these two |
| `discovery_token` | `OPENFLARE_DISCOVERY_TOKEN` | auto-registration Token | one of these two |
| `node_name` | `OPENFLARE_NODE_NAME` | node identifier name | local hostname by default |
| `node_ip` | `OPENFLARE_NODE_IP` | node egress/listen IP | auto-detected real egress IP |
| `frps_path` | `OPENFLARE_FRPS_PATH` | frps executable binary path | `"frps"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | local data and generated `frps.toml` directory | `"./data"` |
| `state_path` | - | local state JSON record file path | `"{data_dir}/relay-state.json"` |
| `heartbeat_interval`| - | heartbeat period (ms int or Go Duration string) | `10000` (10s) |
| `request_timeout` | - | API request timeout | `10000` (10s) |

---

## Running with Docker

Docker is the most convenient deployment for a TunnelRelay node. The official image bundles the `openflare-relay` controller and the `frps` runtime — out of the box.

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7000:7000 \
  -p 17500:17500 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  -v openflare-relay-data:/app/data \
  ghcr.io/rain-kl/openflare-relay:latest
```

> [!TIP]
> The `-p 7000:7000` mapping is the port frpc clients connect to for relaying. If the admin panel configures a custom `relay_bind_port`, adjust the host port mapping accordingly.

> [!NOTE]
> **Enable the embedded frps Web UI**:
> If the Server control panel enables the relay traffic monitoring panel (i.e. `relay_frps_web_ui_enabled` set to `true` in DB/system settings), you need to map the Web port (default `17500`, controlled by `relay_frps_web_ui_port` in system settings) to the host via `-p 17500:17500`.
> The Web UI username is fixed to `admin`, and the password is the relay node's `agent_token`.

---

## Startup and Verification

### 1. View Process Logs

```bash
# Docker container logs
docker logs -f openflare-relay
```

### 2. Verify Runtime State

After starting successfully, the Relay will:
- Send HTTP heartbeats to the control plane to register/go online.
- Fetch the latest frps base config from the control plane (including `bindPort`, `vhostHTTPPort`, and the auto-generated tunnel auth credential `auth_token`).
- Render the local `data/frps.toml` config file.
- Spawn the child process `frps -c data/frps.toml`.
- If the process exits unexpectedly, the Relay auto-restarts frps with exponential backoff (initial 1s, cap 60s).

### 3. Confirm in the Admin Panel

Log in to the admin panel, navigate to **「Node Management」**, and confirm:
- The TunnelRelay node status is marked **「Online」**.
- The node type is correctly marked as **Relay node** and the frps runtime state is **Healthy**.
