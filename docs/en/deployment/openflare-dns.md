---
title: OpenFlare DNS (SmartDNS) Deployment
description: Deployment guide for the standalone OpenFlare DNS authoritative server (openflare-dns), including Linux one-step installation, Docker deployment, and remote upgrades.
---

# OpenFlare DNS (SmartDNS) Deployment

`OpenFlare DNS` (`openflare-dns`) is a standalone high-performance authoritative DNS server that works with the OpenFlare control plane to provide delegated subdomain resolution, network-quality-aware traffic steering, and offline snapshot failover.

For console configuration, NS delegation setup, and visual scheduling rules, see [SmartDNS Authoritative Resolution](../guide/smartdns.md).

---

## Release Channels and Images

`openflare-dns` is versioned and published independently from OpenFlare Server and OpenFlare Edge:

| Channel | Tag Format | Docker Image Tags | Description |
| :--- | :--- | :--- | :--- |
| **Stable** | `dns-vX.Y.Z` (for example `dns-v1.0.0`) | `ghcr.io/rain-kl/openflare-dns:latest` and `:dns-vX.Y.Z` | Production releases |
| **Canary** | `dns-vX.Y.Z-canary.<run_number>` | `ghcr.io/rain-kl/openflare-dns:canary` and `:dns-vX.Y.Z-canary.<run_number>` | Preview builds from the `canary` branch |

Release archives are published for Linux `amd64` and `arm64` as `openflare-dns_<VERSION>_linux_<arch>.tar.gz`.

---

## Linux Host Deployment

Open **DNS → SmartDNS → Authoritative Instances** in the console to copy a prefilled command, or run the installer directly on a Linux (`x86_64` / `aarch64`) host with ports `53/udp` and `53/tcp` open:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --discovery-token <DISCOVERY_TOKEN> \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

To install the Canary channel, pass `canary` right after `install-openflare-dns.sh`. If you pre-registered an instance in the console, pass `--token-type instance --token <ACCESS_TOKEN> --instance-id <INSTANCE_ID>`.

The installer runs `openflare-dns` under an unprivileged `openflare-dns` systemd user with `CAP_NET_BIND_SERVICE` and strict filesystem hardening, persists its registered identity with `0600` permissions, and verifies service readiness.

### Remote Upgrades from the Console

Online Linux systemd instances support one-step single or batch remote upgrades directly from **DNS → SmartDNS → Authoritative Instances** (choosing Stable or Canary). The host verifies the downloaded package checksum, restarts the service, checks readiness, and automatically rolls back to the previous binary if the new version fails to start.

Manual host upgrade command:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh | sudo bash
```

---

## Docker Deployment

The official image `ghcr.io/rain-kl/openflare-dns` is a minimal Alpine image running as a non-root user and listening on `0.0.0.0:5353` inside the container:

```bash
docker run -d --name openflare-dns --restart unless-stopped \
  -p 53:5353/udp \
  -p 53:5353/tcp \
  -e OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com \
  -e OPENFLARE_DNS_DISCOVERY_TOKEN=<DISCOVERY_TOKEN> \
  -e OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53 \
  -v openflare_dns_data:/var/lib/openflare-dns \
  ghcr.io/rain-kl/openflare-dns:latest
```

You can also start the `dns` service from the repository's root `docker-compose.yaml`:

```bash
export OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com
export OPENFLARE_DNS_DISCOVERY_TOKEN=<DISCOVERY_TOKEN>
export OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53
docker compose up -d dns
```

For the full list of CLI flags and container environment variables, see the [Chinese OpenFlare DNS Deployment Guide](/deployment/openflare-dns).
