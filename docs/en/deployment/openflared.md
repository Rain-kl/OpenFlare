# Deploy the OpenFlared Client

You will learn: OpenFlared's responsibilities, config parameters and env vars, running the client with Docker, and deploying it standalone on an intranet server via binary.

**OpenFlared** is the tunnel client deployed in your intranet (LAN, private cloud, or any environment not directly reachable from the public internet). Its core responsibility is communicating with the control plane (OpenFlare Server) via `X-Tunnel-Token`, and locally spawning and managing one or more **frpc (fast reverse proxy client)** processes to securely and stably tunnel intranet HTTP traffic to public relay nodes.

---

## Prerequisites

1. **Get a Tunnel Token**: add a node of type **Tunnel** in admin「Node Management」, save it, then open the node detail page to view the dedicated access Token.
2. **Outbound network access**: the intranet server needs no public inbound IP or port mapping, but must reach the public **OpenFlare Server address** and the corresponding **TunnelRelay node relay port (default 7000)** over the network.
3. **Software dependency** (host deployment only):
   - an executable `frpc` binary locally, or an explicitly specified path via parameter.

---

## Config File and Env Vars

`openflared` reads `flared.json` in the current directory by default at startup, fully overridable via env vars.

### Config Field Details

| JSON field | Env var | Description | Default |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server API service address | **none (required)** |
| `tunnel_token` | `OPENFLARE_TUNNEL_TOKEN` | tunnel client's dedicated auth Token | **none (required)** |
| `frpc_path` | `OPENFLARE_FRPC_PATH` | frpc executable binary path | `"frpc"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | local data and generated `frpc_{relayNodeID}.toml` directory | `"./data"` |
| `state_path` | - | local state record file path (last applied config version) | `"{data_dir}/flared-state.json"` |
| `heartbeat_interval`| - | state heartbeat report period (ms int or Go Duration string) | `10000` (10s) |
| `sync_interval` | - | tunnel config pull/sync period (ms int or Go Duration string) | `30000` (30s) |
| `request_timeout` | - | API network request timeout | `10000` (10s) |

---

## Running with Docker

Docker deployment is the simplest and safest way to run in the intranet. The official `openflared` image bundles the client controller and the `frpc v0.69.0` binary runtime — no extra environment needed.

```bash
docker pull ghcr.io/rain-kl/openflared:latest
docker rm -f openflared 2>/dev/null || true

docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_TUNNEL_TOKEN=YOUR_TUNNEL_TOKEN \
  -v openflared-data:/app/data \
  ghcr.io/rain-kl/openflared:latest
```

---

## Startup and Verification

### 1. Auto-Sync Logic

After starting successfully, OpenFlared runs this workflow:
- **Heartbeat & config fetch**: periodically syncs with the Server's `/api/v1/tunnel/heartbeat` and `/api/v1/tunnel/config/active` endpoints, validating the Token and detecting config versions.
- **File rendering**: when a config version (or checksum) changes, it auto-pulls the tunnel's full route rules. If multiple Relay nodes are bound, it renders `frpc_{relayNodeID}.toml` per Relay under `data_dir`.
- **Config-change restart**: when config or checksum changes, it re-spawns the corresponding `frpc` child processes to keep traffic mappings current.
- **Abnormal self-recovery**: if a local `frpc` tunnel process exits abnormally, the supervisor restarts it with exponential backoff (initial 1s, cap 60s).

### 2. View Logs and Connection State

```bash
# Docker container logs
docker logs -f openflared
```

If the process runs correctly, you'll see output like:
```text
flared config loaded ...
detected frpc version v0.69.0
flared process started
applying new tunnel config {"version": "...", "checksum": "..."}
frpc process missing, starting {"relay_id": "..."}
```

### 3. Confirm in the Admin Panel

Open **「Node Management」** in the admin panel and enter the Tunnel node's detail page:
- View the node online state and flared runtime state (WebSocket connected / running / offline).
- View the current applied version and the latest apply record.
