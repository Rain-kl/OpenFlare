# Deploy OpenFlare Edge

OpenFlare Edge (`openflare-edge`) is OpenFlare's edge proxy and security gateway for Linux `amd64`/`arm64` hosts and Docker containers. For the dual-process model, in-memory TLS keys, configuration sync, and self-update internals, see [OpenFlare Edge Architecture Design](../design/pingora-edge-design.md).

## Prerequisites

- **Control plane reachability**: Outbound HTTPS access from the edge host to OpenFlare Server.
- **Inbound ports**: `80/tcp` and `443/tcp` on Linux hosts; containers listen on `8080` and `8443` internally and map to host ports `80` and `443`.
- **Authentication token** (copy a prefilled install command from **Node Management** > node details > **Edge Node Deployment**):
  - **Node token (`node`)**: Bound to a pre-created node in **Node Management** (recommended for production).
  - **Discovery token (`discovery`)**: Configured under **System Settings** > **OpenFlare** to auto-register new nodes on startup.

## Linux host installation

### Installer commands

Install with a dedicated node token (`--token-type node`) to start `openflare-edge.service`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token '<NODE_TOKEN>' \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

To auto-register with a global discovery token, pass `--token-type discovery`:

```bash
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type discovery \
  --token '<DISCOVERY_TOKEN>'
```

From a cloned repository or extracted archive, run `sudo bash scripts/install-openflare-edge.sh ...`.

### Installer options

| Option | Description | Default |
| --- | --- | --- |
| `--server-url URL` | OpenFlare Server URL (required) | None |
| `--token TOKEN` | Node or discovery token (alternative to `--token-file`) | None |
| `--token-file PATH` | Read token from a local file | None |
| `--token-type TYPE` | Token type: `node` or `discovery` | `discovery` |
| `--version TAG` | Specific `edge-v*` release tag (e.g. `edge-v0.1.6`) | Latest stable |
| `--repo OWNER/NAME` | GitHub release repository | `Rain-kl/OpenFlare` |
| `--http-listen ADDR` | HTTP listen address | `0.0.0.0:80` |
| `--tls-listen ADDR` | HTTPS/TLS listen address (`none` disables TLS) | `0.0.0.0:443` |
| `--install-dir DIR` | Binary installation directory | `/opt/openflare-edge` |
| `--state-dir DIR` | State and snapshot directory | `/var/lib/openflare-edge` |
| `--no-service` | Install files without creating systemd units | `false` |

## Docker container deployment

Run the static container image `ghcr.io/rain-kl/openflare-edge` (`:latest` or `:canary`):

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  --cap-add=NET_RAW \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<NODE_TOKEN>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

Container options:

- **Keep `--cap-add=NET_RAW`**: Required for the isolated probe worker to run native ICMP ping and traceroute checks.
- **Mounted token file**: Set `OPENFLARE_EDGE_TOKEN_FILE` (default `/run/secrets/openflare-edge-token`, readable by UID `10001`) instead of `OPENFLARE_EDGE_TOKEN`. The two are mutually exclusive.
- **Listeners and state path**: Override `OPENFLARE_EDGE_HTTP_LISTEN` (`0.0.0.0:8080`), `OPENFLARE_EDGE_TLS_LISTEN` (`0.0.0.0:8443`, or `none` to disable TLS), and `OPENFLARE_EDGE_STATE_DIR` (`/var/lib/openflare-edge`) as needed; mount a volume at `/var/lib/openflare-edge` to persist identity and snapshots.

## Verification and uninstallation

Check host or container status:

```bash
# Linux systemd host
/opt/openflare-edge/current --version
sudo systemctl status openflare-edge
sudo journalctl -u openflare-edge -n 50 --no-pager

# Docker container
docker logs -f openflare-edge
```

In the console, confirm the node appears online under **Nodes** > **Node List** with its active configuration version, then bind a policy under **Performance policies**.

Stop and remove the systemd service and binary (append `--remove-state` to also delete state files and GeoIP databases):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-edge.sh | sudo bash
```

For upgrades and rollbacks, see [Upgrades & maintenance](./upgrade.md).
