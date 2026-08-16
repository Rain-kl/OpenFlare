# Deploy OpenFlared Client

You will learn: The responsibilities of the OpenFlared client, configuration parameters and environment variables, how to run the client via Docker, and how to deploy the client on an intranet server using the compiled host binary.

**OpenFlared** is a tunnel client deployed in the user's intranet environment (LANs, private VPCs, or other environments that cannot be directly accessed from the public internet). Its core responsibility is to establish communication with the control plane (OpenFlare Server) via the `X-Tunnel-Token` header, automatically spawning and managing one or more **frpc (Fast Reverse Proxy Client)** subprocesses locally to securely and stably tunnel HTTP traffic back to public relay nodes.

---

## Prerequisites

1. **Retrieve Tunnel Token**: Create a new tunnel instance on the "Intranet Penetration" or "Tunnel Management" page in the OpenFlare management console; the system will automatically generate a unique `tunnel_id` and a `tunnel_token` (e.g., `tun-<32hex>`).
2. **Outbound Network Permissions**: The intranet server does not require any inbound public IPs or port mappings, but it must be able to reach the **OpenFlare Server URL** and the corresponding **TunnelRelay node control port (default 7000)** over the outbound network.
3. **Software Dependencies** (Host deployment only):
   - You must have an executable `frpc` binary locally (recommended version `v0.61.0+` or the latest stable `v0.69.0`), or specify its path explicitly in the configuration.

---

## Configuration & Environment Variables

`openflared` reads `flared.json` in the working directory by default on startup. Overriding options via environment variables is fully supported.

### Configuration Fields Details

| JSON Field | Environment Variable | Description | Default Value |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server API base URL | **None (Required)** |
| `tunnel_token` | `OPENFLARE_TUNNEL_TOKEN` | Tunnel client dedicated access Token | **None (Required)** |
| `frpc_path` | `OPENFLARE_FRPC_PATH` | Path to the `frpc` executable binary | `"frpc"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | Directory to store local data and generated `frpc_{relayNodeID}.toml` configs | `"./data"` |
| `state_path` | - | Path to store local state JSON file (saving the last applied version) | `"{data_dir}/flared-state.json"` |
| `heartbeat_interval`| - | Heartbeat reporting interval (ms or Go Duration string) | `10000` (10s) |
| `sync_interval` | - | Tunnel config polling interval (ms or Go Duration string) | `30000` (30s) |
| `request_timeout` | - | HTTP request timeout duration | `10000` (10s) |

---

## Docker Deployment (Recommended)

Docker is the simplest and safest way to run the client inside the intranet. The official `openflared` image embeds the client controller and `frpc v0.69.0` out of the box, requiring no environment setup.

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

## Manual Host Deployment

If you need to run the client directly on a Linux/macOS/Windows host inside the intranet:

### 1. Compile the Binary

```bash
cd openflared
go build -o flared ./cmd/flared
```

### 2. Prepare `flared.json`

Create a `flared.json` configuration file in the same directory as the executable:

```json
{
  "server_url": "http://your-server-ip:3000",
  "tunnel_token": "your-tunnel-auth-token",
  "frpc_path": "/usr/local/bin/frpc",
  "data_dir": "./data",
  "heartbeat_interval": "10s",
  "sync_interval": "30s"
}
```

### 3. Start the Service

```bash
export LOG_LEVEL='info'
./flared -config ./flared.json
```

---

## Start & Validate

### 1. Auto-Sync Workflow

Once started successfully, OpenFlared operates the following workflow:
- **Heartbeat & Config Fetching**: Periodically polls `/api/flared/heartbeat` and `/api/flared/config` endpoints to validate the Token and evaluate configuration versions.
- **File Rendering**: When a new configuration version (or checksum mismatch) is detected, it pulls the complete tunnel routing rules. If multiple Relays are bound, it renders `frpc_{relayNodeID}.toml` configurations in `data_dir` for each Relay.
- **Hot Reload or Restart**: Spawns the corresponding `frpc` subprocesses, or executes `frpc reload` / restart actions when configurations change, ensuring traffic mappings are kept up to date.
- **Process Auto-Recovery**: If a local `frpc` tunnel process exits unexpectedly, the master program automatically restarts it after a 5-second backoff penalty.

### 2. View Logs & Connection Status

```bash
# Docker container logs
docker logs -f openflared
```

If running correctly, the logs will show output similar to:
```text
flared config loaded ...
detected frpc version v0.69.0
flared process started
applying new tunnel config {"version": "...", "checksum": "..."}
frpc process missing, starting {"relay_id": "..."}
```

### 3. Verify in the Management Console

Open the **"Intranet Penetration"** page in the management console:
- Check the online status of the corresponding tunnel; it should display green as **"Online"**.
- You can inspect which relay nodes the tunnel is connected to, and view the detailed routing configurations of the intranet services.
