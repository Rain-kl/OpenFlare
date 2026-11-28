# Deployment planning & baselines

OpenFlare separates its control plane from its data plane. For network traffic flows across components, see [System Architecture](../design/architecture.md).

## Runtime dependencies

- **Redis / Valkey (required)**: Required in all deployment modes for Asynq queues, cache invalidation, and multi-replica broadcasts.
- **Primary database**: Supports embedded SQLite for evaluation or single-host setups, and PostgreSQL for production workloads.
- **ClickHouse (optional)**: Offloads high-volume access logs and telemetry; falls back to the primary database when disabled (see [Log Store Decoupling](../design/logstore.md)).
- **Session secret**: Set a long random `APP_SESSION_SECRET` (at least 32 bytes for MCP) before exposing the Server.

## Network port matrix

| Component | Ports and protocols | Exposure | Purpose |
| --- | --- | --- | --- |
| OpenFlare Server | `3000/tcp` | Inbound (admin UI and nodes) | Console UI, Admin API, and node endpoints (configurable via `APP_ADDR`) |
| PostgreSQL | `5432/tcp` | Internal to Server | Primary relational database |
| Redis / Valkey | `6379/tcp` | Internal to Server | Asynq queues and cross-instance pub/sub |
| ClickHouse (optional) | `9000/tcp`, `8123/tcp` | Internal to Server | Analytical log and observability storage |
| Jaeger (optional) | `16686/tcp`, `4317/tcp`, `4318/tcp` | `16686` internal UI; `4317`/`4318` from Server | OpenTelemetry trace ingestion and UI |
| OpenFlare Edge | `80/tcp`, `443/tcp` | Public inbound (`8080`, `8443` in container) | HTTP and HTTPS/TLS entry listeners |
| OpenFlare DNS | `53/udp`, `53/tcp` | Public inbound (`5353` in container) | Authoritative DNS query listener |
| OpenFlare Relay | `7443/tcp`, `7444/tcp` | Public inbound (from `openflared`) | `rathole` Noise/TCP (`7443`) and WebSocket (`7444`) transports |
| OpenFlare Relay | `8443/tcp` | Reachable by Edge and `openflared` | Relay HTTP/WS Gateway and `/_openflare/health` probes |
| OpenFlare Relay (L4) | Per service | Public inbound | Assigned L4 TCP/UDP tunnel ports |
| OpenFlared | No inbound ports | Outbound only | Connects to Server and healthy pool relays |

## Hardware baselines

| Component | Entry level | Production recommended | Notes |
| --- | --- | --- | --- |
| OpenFlare Server | 1 vCPU / 2 GB RAM / 20 GB disk | 2 vCPU / 4 GB RAM / 50 GB+ disk | Scale disk by database growth and log retention |
| OpenFlare Edge | 1 vCPU / 512 MB RAM / 2 GB disk | 2 vCPU / 2 GB RAM / 10 GB+ disk | Scale by concurrent connections, TLS handshakes, WAF load, and disk cache |
| OpenFlare DNS | 1 vCPU / 256 MB RAM / 1 GB disk | 2 vCPU / 512 MB RAM / 5 GB disk | Memory holds zone snapshots, answer cache, and GeoIP databases |
| OpenFlare Relay | 1 vCPU / 1 GB RAM / 5 GB disk | 2 vCPU / 2 GB RAM / 20 GB disk | Throughput depends on network bandwidth and concurrent streams |
| OpenFlared | 1 vCPU / 256 MB RAM / 1 GB disk | 1 vCPU / 512 MB RAM / 5 GB disk | Runs on private hosts with low overhead |

## Component deployment guides

- Control plane Compose stacks and source builds: [Server deployment](./server.md)
- Edge proxy gateway: [Deploy OpenFlare Edge](./openflare-edge.md)
- Standalone authoritative DNS: [Deploy OpenFlare DNS](./openflare-dns.md)
- Tunnel relay and connector: [Deploy OpenFlare Relay](./relay.md), [Deploy OpenFlared](./openflared.md)
- Upgrades and rollbacks: [Upgrades & maintenance](./upgrade.md)
