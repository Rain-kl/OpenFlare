# Deploy OpenFlare DNS

OpenFlare DNS (`openflare-dns`) is the standalone Rust authoritative DNS node for SmartDNS, supporting Linux `amd64`/`arm64` hosts and Docker containers. For scheduling internals and ECS scoring, see [SmartDNS System Architecture Design](../design/smartdns-design.md); for console setup, see [SmartDNS Guide](../guide/smartdns.md).

## Prerequisites

- **Control plane reachability**: Outbound HTTPS access to OpenFlare Server.
- **Inbound ports**: `53/udp` and `53/tcp` on Linux hosts; containers listen on `5353` internally and map to host port `53` (or use `--network host` with `--listen 0.0.0.0:53`).
- **Authentication token**: Copy a prefilled command from **DNS** > **SmartDNS** > **Authoritative Instances** using either a `discovery` token or a pre-registered `instance` token.

## Linux host installation

### Installer commands

Install the stable channel using a discovery token (append `canary` for preview builds):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --discovery-token '<DISCOVERY_TOKEN>' \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

For a pre-registered instance, pass `--token-type instance --token '<ACCESS_TOKEN>' --instance-id '<INSTANCE_ID>'`.

### Installer options

| Option | Description | Default |
| --- | --- | --- |
| `canary` / `--canary` / `--channel` | Release channel (`stable` or `canary`) | `stable` |
| `--controller-url` | OpenFlare control plane URL (required) | None |
| `--discovery-token` / `--token` / `--token-file` | Discovery token, instance token, or token file path | None |
| `--token-type` | Token type: `discovery` or `instance` | `discovery` |
| `--instance-id` / `--name` | Explicit instance ID and display name | Auto-generated |
| `--public-addr` | Public IP or hostname announced by the node | None |
| `--listen` | UDP and TCP listen address | `0.0.0.0:53` |
| `--workers` | `SO_REUSEPORT` UDP worker thread count (`0` for auto-detect) | `0` |
| `--version` | Specific `dns-v*` release tag (exclusive with `canary`) | Latest stable |
| `--install-dir` / `--state-dir` / `--geoip-dir` | Binary, state, and GeoIP MMDB directories | `/opt/openflare-dns`, `/var/lib/openflare-dns`, `<state-dir>/geoip` |
| `--skip-geoip` / `--no-service` | Skip GeoIP download or systemd unit creation | `false` |

### Enable remote console updates

Fresh installations register `openflare-dns-update.path` and `openflare-dns-update.service`. On older Linux hosts, run the updater with `--enable-remote` to allow single or batch upgrades from the console:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh -o update-openflare-dns.sh
sudo bash update-openflare-dns.sh --enable-remote
sudo systemctl restart openflare-dns
```

## Docker container deployment

Run `ghcr.io/rain-kl/openflare-dns` (`:latest` or `:canary`):

```bash
docker run -d --name openflare-dns --restart unless-stopped \
  -p 53:5353/udp \
  -p 53:5353/tcp \
  -e OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com \
  -e OPENFLARE_DNS_DISCOVERY_TOKEN='<DISCOVERY_TOKEN>' \
  -e OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53 \
  -v openflare_dns_data:/var/lib/openflare-dns \
  ghcr.io/rain-kl/openflare-dns:latest
```

With the repository root `docker-compose.yaml`, export these variables and run `docker compose up -d dns`. Full container environment variables are listed in [Configuration Reference](../reference/configuration.md).

## Verification and uninstallation

Verify service status and authoritative (`aa`) responses:

```bash
sudo systemctl status openflare-dns
dig @203.0.113.53 sdns.example.com NS +norecurse
```

Uninstall the systemd service and binary (append `--remove-state` to remove credentials, snapshots, and GeoIP data):

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-dns.sh | sudo bash
```

For upgrades and rollbacks, see [Upgrades & maintenance](./upgrade.md).
