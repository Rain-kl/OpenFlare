# Deploy Relay (Tunnel Relay)

You will learn: The responsibilities of a TunnelRelay node, `openflare-relay` configuration parameters and environment variables, how to run the Relay via Docker, and how to build and deploy the Relay from source manually.

In the OpenFlare intranet penetration architecture, the **TunnelRelay node** plays a key role. Unlike standard Edge Nodes, in addition to running the traditional Agent (managing OpenResty for HTTPS/WAF processing), it co-locates the **Relay (frps tunnel manager)** service, responsible for listening to intranet client (OpenFlared) tunnel connections and relaying traffic.

---

## Prerequisites

Before deploying a TunnelRelay node, ensure:

1. **Registered as a TunnelRelay node**: Add a node of type `tunnel_relay` in the OpenFlare management console under "Node Management", and retrieve its node-specific `agent_token` or use the global `discovery_token`.
2. **Network Ports**:
   - Ensure `bindPort` (the port frpc clients connect to, default `7000`) is accessible from the public/intranet client networks.
   - Ensure `vhostHTTPPort` (the HTTP Vhost port, default `8080`) is free and not bound by other processes, as the Agent routes traffic to frps on this port.
3. **Software Dependencies** (Host deployment only):
   - You must have an executable `frps` binary locally (recommended version `v0.61.0+` or the latest stable `v0.69.0`), or specify its path explicitly in the configuration.

---

## Configuration & Environment Variables

`openflare-relay` reads `relay.json` in the working directory by default on startup. Overriding options via environment variables is fully supported.

### Configuration Fields Details

| JSON Field | Environment Variable | Description | Default Value |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server API base URL | **None (Required)** |
| `agent_token` | `OPENFLARE_AGENT_TOKEN` | Node-specific Token | Mutually exclusive with below |
| `discovery_token` | `OPENFLARE_DISCOVERY_TOKEN` | One-time auto-registration Token | Mutually exclusive with above |
| `node_name` | `OPENFLARE_NODE_NAME` | Custom name for the node | Hostname by default |
| `node_ip` | `OPENFLARE_NODE_IP` | Outbound/listening IP of the node | Automatically detects real outbound IP |
| `frps_path` | `OPENFLARE_FRPS_PATH` | Path to the `frps` executable binary | `"frps"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | Directory to store local data and generated `frps.toml` | `"./data"` |
| `state_path` | - | Path to store local state JSON file | `"{data_dir}/relay-state.json"` |
| `heartbeat_interval`| - | Heartbeat interval (integer ms or Go Duration string) | `10000` (10s) |
| `request_timeout` | - | HTTP request timeout duration | `10000` (10s) |

---

## Docker Deployment (Recommended)

Docker is the most convenient way to deploy a TunnelRelay node. The official Docker image embeds the `openflare-relay` controller and `frps v0.69.0` out of the box.

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7000:7000 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  -v openflare-relay-data:/var/lib/openflare-relay \
  ghcr.io/rain-kl/openflare-relay:latest
```

> [!TIP]
> The `-p 7000:7000` option maps the port `frpc` clients connect to. If a custom `relay_bind_port` is configured in the management console, change this port mapping on the host accordingly.

---

## Manual Host Deployment

If you prefer to run the Relay directly on a physical host or VM:

### 1. Compile the Binary

```bash
cd openflare-relay
go build -o openflare-relay ./cmd/relay
```

### 2. Prepare `relay.json`

Create a `relay.json` configuration file in the same directory as the executable:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "your-relay-node-agent-token",
  "frps_path": "/usr/local/bin/frps",
  "data_dir": "./data",
  "heartbeat_interval": "10s",
  "request_timeout": "10s"
}
```

### 3. Start the Service

```bash
export LOG_LEVEL='info'
./openflare-relay -config ./relay.json
```

---

## Start & Validate

### 1. View Process Logs

```bash
# Docker container logs
docker logs -f openflare-relay
```

If managed via systemd on Linux, execute:
```bash
journalctl -u openflare-relay -f
```

### 2. Verify Runtime Status

Upon starting successfully, the Relay operates as follows:
- Sends HTTP heartbeats to register and go online with the control plane.
- Retrieves the active frps baseline settings (including `bindPort`, `vhostHTTPPort`, and the auto-generated `auth_token`).
- Automatically renders the `data/frps.toml` configuration locally.
- Spawns the subprocess `frps -c data/frps.toml`.
- If the `frps` process crashes, the Relay automatically restarts it after 2 seconds.

### 3. Verify in the Management Console

Log into the management console and navigate to **"Node Management"** to verify:
- The TunnelRelay node status is marked as **"Online"**.
- The Node Type is correctly displayed as **Relay Node** and the frps status displays as **Healthy**.
