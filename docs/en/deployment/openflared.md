# Deploy OpenFlared

`openflared` is the intranet tunnel connector built in Rust with an embedded `rathole` client, supporting active-active multi-instance deployment. For failover and CIDR SSRF protection, see [Tunnel & Intranet Penetration Design](../design/tunnel-design.md); for publishing L7/L4 services, see [Tunnels & Intranet Penetration](../guide/tunnel-usage.md).

## Prerequisites

- **Bootstrap token**: Create a tunnel under **Tunnels** > **Tunnels** and copy its `oftok1_` token or install command.
- **Outbound connectivity**: No inbound ports are required; the host needs outbound access to OpenFlare Server and pool relays on `7443/tcp` (or `7444/tcp`) and `8443/tcp`.

## Linux host deployment

Install the static binary and `openflared.service`:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflared.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

Run the same command on separate hosts for redundancy, or pass distinct `--state-dir` and `--connector-id` values when running multiple instances on one host. Common options (full flags and environment variables are in [Configuration Reference](../reference/configuration.md)):

- `--server-url URL`: Control plane URL when not using an `oftok1_` token.
- `--token-file PATH`: Read token from a restricted file.
- `--connector-id ID`: Explicit persistent connector ID (auto-generated UUID by default).
- `--version tunnel-vX.Y.Z` / `--install-dir` / `--state-dir` / `--no-service`: Override release tag, directories, or systemd setup.

## Docker container deployment

Run `ghcr.io/rain-kl/openflared` (non-root UID/GID `10001`):

```bash
docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_TUNNEL_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflared-data:/var/lib/openflared \
  ghcr.io/rain-kl/openflared:latest
```

On Linux hosts, pass `--network host` to reach services bound to `127.0.0.1`; in bridge mode, target services by container name or permitted LAN IP.

## Verification and uninstallation

Check logs and confirm the connector and **Attachments** appear online under **Tunnels** > **Tunnel Details**:

```bash
sudo systemctl status openflared
sudo journalctl -u openflared -n 50 --no-pager
```

Uninstall the systemd service (append `--remove-state` to delete connector ID and state files):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflared.sh | sudo bash
```

For upgrades and rollbacks, see [Upgrades & maintenance](./upgrade.md).
