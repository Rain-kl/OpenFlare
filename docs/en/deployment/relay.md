# Deploy OpenFlare Relay

`openflare-relay` is the public Rust tunnel relay daemon with an embedded `rathole` engine. It terminates encrypted tunnels from `openflared`, serves the L7 Relay Gateway, and forwards L4 TCP/UDP ports. For protocol and scheduling internals, see [Tunnel & Intranet Penetration Design](../design/tunnel-design.md).

## Prerequisites

- **Bootstrap token**: Open **Tunnels** > **Relays** in the console and copy the global auto-discovery command or a pool-specific `oftok1_` token.
- **Inbound ports**:
  - `7443/tcp`: `rathole` Noise/TCP transport port (reachable from `openflared`).
  - `7444/tcp`: `rathole` WebSocket fallback port.
  - `8443/tcp`: Relay Gateway and `/_openflare/health` probe port (reachable from Edge nodes and `openflared`).
  - **L4 ports**: Open any TCP/UDP ports assigned to L4 tunnel services.
- **Zones and colocation (recommended)**: Set `zone_tag` for failure-domain distribution and `colocated_edge_node_id` when running alongside an Edge node.

## Linux host deployment

`openflare-relay` and `openflared` share the `tunnel-v*` release line (`amd64`/`arm64`). Install using an `oftok1_` token (which embeds the control plane URL):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-relay.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

Common installer options (full flags and environment variables are in [Configuration Reference](../reference/configuration.md)):

- `--server-url URL`: Control plane URL (optional when using an `oftok1_` token).
- `--public-addr HOST_OR_IP` / `--gateway-addr HOST_OR_IP`: Explicit public or Edge-facing gateway address.
- `--zone-tag TAG` / `--colocated-edge-node-id ID`: Failure domain tag and colocated Edge node ID.
- `--transport-port` / `--ws-port` / `--gateway-port`: Custom listen ports (defaults `7443`, `7444`, `8443`).
- `--version tunnel-vX.Y.Z` / `--install-dir` / `--state-dir` / `--no-service`: Override release tag, directories, or systemd setup.

## Docker container deployment

Run `ghcr.io/rain-kl/openflare-relay` (non-root UID/GID `10001`):

```bash
docker run -d --name openflare-relay --restart unless-stopped \
  -p 7443:7443 \
  -p 7444:7444 \
  -p 8443:8443 \
  -e OPENFLARE_RELAY_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflare-relay-data:/var/lib/openflare-relay \
  ghcr.io/rain-kl/openflare-relay:latest
```

Publish additional `-p` mappings (or use `--network host`) when exposing L4 TCP/UDP services.

## Verification and uninstallation

Inspect service logs and verify **Online** status under **Tunnels** > **Relays**:

```bash
sudo systemctl status openflare-relay
sudo journalctl -u openflare-relay -n 50 --no-pager
```

Uninstall the systemd service (append `--remove-state` to delete identity and state files):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-relay.sh | sudo bash
```

For upgrades and two-slot hot updates, see [Upgrades & maintenance](./upgrade.md).
