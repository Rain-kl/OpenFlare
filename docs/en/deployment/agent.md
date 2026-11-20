# Edge node quick access

Enrolling Pingora Edge nodes requires a node token or a discovery token. For packaging specifications, systemd low-privilege security directives, the dual-process supervisor model, and atomic updates, see [OpenFlare Edge deployment](./openflare-edge.md).

## Prerequisites

Each edge node must meet the following network requirements:
- Control plane connectivity: Outbound HTTPS access to the OpenFlare Server address.
- Inbound listener ports: Ports `80` (HTTP) and `443` (HTTPS) reachable by visitor traffic.
- Node authentication token:
  - Node token: Generated in the admin console under Node Management. The node detail page displays a pre-filled installation command with this dedicated token (recommended for production).
  - Discovery token: Generated in system settings to allow automatic registration on initial node boot.

## Quick installation commands

### Linux host installation

```bash
# Download and execute the installer using a dedicated node token
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type node \
  --token '<node-token>'
```

To enroll with a discovery token, set `--token-type discovery`:

```bash
sudo bash install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type discovery \
  --token '<discovery-token>'
```

When executing from a cloned repository, run the local script:

```bash
sudo bash scripts/install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type node \
  --token '<node-token>'
```

### Common installation options

| Option | Description | Default |
| --- | --- | --- |
| `--server-url URL` | OpenFlare Server control plane URL (required) | None |
| `--token TOKEN` | Authentication token (alternative to `--token-file`) | None |
| `--token-file PATH` | Read token from file path | None |
| `--token-type TYPE` | Token type: `node` or `discovery` | `discovery` |
| `--version TAG` | Specific Edge release tag to install (e.g. `edge-v0.1.3`) | Latest compatible release |
| `--http-listen ADDR` | HTTP listener address | `0.0.0.0:80` |
| `--tls-listen ADDR` | HTTPS/TLS listener address (use `none` to disable TLS) | `0.0.0.0:443` |
| `--install-dir DIR` | Target binary installation directory | `/opt/openflare-edge` |
| `--state-dir DIR` | State and snapshot directory | `/var/lib/openflare-edge` |
| `--no-service` | Extract archive and write files without creating systemd units | `false` |

### Docker quick start

Copy the pre-filled command from the node details page in the console, or run:

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://openflare.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<node-token>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

## Runtime verification

Check systemd service status and logs on the edge host:

```bash
# Check service status
sudo systemctl status openflare-edge

# Stream runtime service logs
sudo journalctl -u openflare-edge -f
```

Admin console verification:
1. Open the OpenFlare admin console and navigate to Node Management.
2. Confirm the edge node is listed as online, reporting its active configuration version and Pingora runtime state.
3. Open [Performance policies](../guide/performance-policies.md) to allocate a profile matching the host hardware.

To remove the service, execute `scripts/uninstall-openflare-edge.sh`. For updates, see the [Upgrade & maintenance guide](./upgrade.md).
