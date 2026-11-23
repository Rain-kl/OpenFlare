# Architecture & Deployment Notes

OpenFlare uses a decoupled architecture separating the control plane from the data plane. Production environments should use PostgreSQL as the primary database for the Server, with `APP_SESSION_SECRET` configured via configuration files or environment variables. Redis is required to drive the Asynq background task system; for high-volume access logs and metrics, ClickHouse can be integrated (see [Log Store Decoupling](../design/logstore.md)). The edge gateway (OpenFlare Edge) is built on OpenFlare Edge, running a resilient dual-process architecture.

## Network Topology and Traffic Paths

### 1. Standard Reverse Proxy Traffic Path

Control plane and data plane traffic are isolated: user client requests flow directly to the OpenFlare Edge gateway, while the Server only distributes configuration snapshots and collects telemetry.

```text
[Control Plane Flow]
OpenFlare Server (Control plane, listens on :3000)
       ^
       | Periodic snapshot pull / Heartbeat / Log telemetry (HTTPS)
       v
OpenFlare Edge (Data plane gateway)

[Data Plane Flow]
User Client / Browser
       |
       v Inbound request (TCP 80 / 443)
OpenFlare Edge (OpenFlare Edge: TLS termination, WAF filtering, caching, Pages hosting)
       |
       v Reverse proxy origin fetch (HTTP / HTTPS)
Origin Service
```

### 2. Intranet Penetration Traffic Path

Intranet penetration operates across the edge gateway, a public relay host, and the intranet client:

```text
[Control Plane Flow]
OpenFlare Server (Control plane)
    ^                             ^
    | Config sync & heartbeat      | Tunnel token validation & heartbeat
    v                             v
Relay Node (frps supervisor)    OpenFlared (Intranet client)

[Data Plane Flow]
User Client / Browser
    |
    v Public access (TCP 80 / 443)
OpenFlare Edge (OpenFlare Edge: Host routing, WAF evaluation, TLS termination)
    |
    v Local loopback forward (127.0.0.1:{vhostHTTPPort})
OpenFlare Relay (frps daemon, listens on public relay port :7000)
    |
    | Encrypted FRP Tunnel (outbound connection from client)
    v
OpenFlared Client (frpc daemon, runs on private host)
    |
    v LAN forwarding (Internal IP / port)
Internal Service (192.168.x.x)
```

## Hardware Baseline

| Component | Entry Level | Production Recommended | Notes |
| --- | --- | --- | --- |
| **Server Control Plane** | 1 core CPU / 2 GB RAM / 20 GB disk | 2 cores CPU / 4 GB RAM / 50 GB+ disk | Scale disk capacity based on log retention policies and metric volume |
| **Edge Gateway** | 1 core CPU / 512 MB RAM / 2 GB disk | 2 cores CPU / 2 GB RAM / 10 GB+ disk | Scale by OpenFlare Edge concurrent proxy connections, TLS handshakes, and WAF rules |
| **Relay Node** | 1 core CPU / 1 GB RAM / 5 GB disk | 2 cores CPU / 2 GB RAM / 20 GB disk | frps throughput is bounded by network bandwidth and TCP socket limits |
| **OpenFlared Client** | 1 core CPU / 256 MB RAM / 1 GB disk | 1 core CPU / 512 MB RAM / 5 GB disk | Runs on private host; minimal CPU/RAM overhead, depends on local network capacity |

## Quick Deployment Reference

### Docker Compose Server Deployment

The repository root includes a standard orchestration file `docker-compose.yaml` with PostgreSQL, Redis, ClickHouse, and Jaeger:

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# Edit .env, configure APP_SESSION_SECRET and database passwords
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

Access `http://localhost:3000` in your browser. The default administrator credentials are `admin` / `12345678`. Change the password in the admin console immediately upon first login. For configuration details, see [Server Deployment](./server.md).

### Source Build Server Startup

To compile and execute from source:

```bash
# 1. Build embedded admin frontend static bundle
cd frontend
corepack enable
pnpm install
pnpm build:embed
cd ..

# 2. Prepare configuration and start server
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
go run ./cmd/server all
```

The server listens on port `:3000` by default (configurable via `app.addr` in `config.yaml` or `APP_ADDR`).

### Deploying Edge and Tunnel Components

- Edge gateway deployment: See [OpenFlare Edge Deployment](./openflare-edge.md) (system specifications, systemd security hardening, atomic updates) or [Edge Node Quick Access](./agent.md) (tokens and quick install flags).
- Tunnel infrastructure deployment: See [Relay Deployment](./relay.md) and [OpenFlared Deployment](./openflared.md).
