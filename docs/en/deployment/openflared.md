# Deploy OpenFlared client

OpenFlared is the tunnel client software deployed within private networks (such as LANs, private subnets, or servers without public IPs). OpenFlared maintains a session with the OpenFlare Server control plane via `X-Tunnel-Token`, pulls routing rules, and supervises local `frpc` (Fast Reverse Proxy Client) child processes to tunnel private HTTP/TCP workloads through public relay nodes.

---

## Prerequisites

1. Obtain tunnel token: In the OpenFlare admin console under Node Management, create a node of type Tunnel and copy its `tunnel_token`.
2. Outbound connectivity: The private server requires no public inbound ports, but must be able to establish outbound connections to the OpenFlare Server control plane API and the bound TunnelRelay public relay port (default `7000`).
3. Software dependency (bare-metal host deployment only): An executable `frpc` binary available on the host system.

---

## Configuration and environment variables

`openflared` reads `flared.json` in the working directory by default. All settings can be overridden via environment variables:

| JSON field | Environment variable | Description | Default |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server control plane endpoint | **Required** |
| `tunnel_token` | `OPENFLARE_TUNNEL_TOKEN` | Dedicated tunnel client authentication token | **Required** |
| `frpc_path` | `OPENFLARE_FRPC_PATH` | Path to executable `frpc` binary | `"frpc"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | Working directory for generated `frpc_{relayNodeID}.toml` | `"./data"` |
| `state_path` | - | Path to local state file (stores last applied version) | `"{data_dir}/flared-state.json"` |
| `heartbeat_interval`| - | Heartbeat interval to Server (milliseconds or Go Duration) | `10000` (10s) |
| `sync_interval` | - | Rule polling and synchronization interval (milliseconds or Go Duration) | `30000` (30s) |
| `request_timeout` | - | HTTP API request timeout (milliseconds or Go Duration) | `10000` (10s) |

---

## Docker container deployment

The official Docker image includes the client controller and `frpc`:

```bash
docker pull ghcr.io/rain-kl/openflared:latest
docker rm -f openflared 2>/dev/null || true

docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_TUNNEL_TOKEN=YOUR_TUNNEL_TOKEN \
  -v openflared-data:/app/data \
  ghcr.io/rain-kl/openflared:latest
```

To proxy local host services (e.g., `http://127.0.0.1:8000` on the host machine), run with `--network host` on Linux, or reference `host.docker.internal:8000` under the default bridge network.

---

## Runtime verification

### Synchronization lifecycle

OpenFlared executes the following automated lifecycle:
- Heartbeat and status polling: Periodically requests the Server endpoints `/api/v1/tunnel/heartbeat` and `/api/v1/tunnel/config/active`, verifying token validity and checking configuration checksums.
- Configuration rendering: When configuration changes are detected, OpenFlared retrieves the full routing table. If bound to multiple Relay nodes, it renders an independent `frpc_{relayNodeID}.toml` for each relay under `data_dir`.
- Process reloading: Spawns or reloads corresponding `frpc` child processes to activate updated route definitions.
- Fault recovery: If an `frpc` process terminates unexpectedly, the supervisor restarts it with exponential backoff (1-second initial delay, capped at 60 seconds).

### View service logs

```bash
docker logs -f openflared
```

Expected log output during normal operation:

```text
flared config loaded ...
detected frpc version v0.69.0
flared process started
applying new tunnel config {"version": "...", "checksum": "..."}
frpc process missing, starting {"relay_id": "..."}
```

### Admin console verification

Open the OpenFlare admin console, navigate to Node Management, and select the tunnel node:
- Confirm the node status indicates online.
- Confirm the synced version identifier and recent synchronization timestamp.
