# System Architecture

OpenFlare comprises the Server control plane, the OpenFlare Edge data plane, standalone OpenFlare DNS authoritative nodes, native Rust tunnel components (`openflare-relay` and `openflared`), and the admin console frontend. For edge synchronization and runtime internals, see [OpenFlare Edge Architecture Design](./pingora-edge-design.md).

## Data Plane Traffic Paths

OpenFlare supports four core data-plane traffic paths:

### 1. Standard Reverse Proxy Path

```text
Browser
  | HTTPS / HTTP
  v
OpenFlare Edge (TLS / WAF / PoW / Rate Limit / EdgeCache / Origin Error Page)
  | HTTP / HTTPS
  v
Origin Server (public or private upstream)
```

### 2. L7 Intranet Tunnel Path (Edge + Relay Gateway)

```text
Browser
  | HTTPS / HTTP / WS
  v
OpenFlare Edge (tunnel-routing.v1, TLS / WAF / PoW)
  | HTTP/WS (X-OpenFlare-Gateway-Auth, nearest healthy relay)
  v
openflare-relay Gateway (:8443)
  | Embedded rathole server (Noise :7443 / WebSocket :7444 / TCP)
  v
openflared (embedded rathole client + CIDR SSRF validation)
  |
  v
Internal Service (private HTTP / HTTPS / WS origin)
```

### 3. L4 TCP/UDP Intranet Tunnel Path

```text
TCP / UDP Client
  |
  v
openflare-relay (assigned public TCP/UDP port)
  | Embedded rathole encrypted tunnel (Noise / WebSocket / TCP)
  v
openflared (CIDR SSRF validation)
  |
  v
Internal TCP / UDP Target
```

### 4. Pages Static Hosting Path

```text
Browser
  | HTTPS / HTTP
  v
OpenFlare Edge (TLS / WAF / PoW / Site Auth)
  |
  +---> [Static serving] local release directory (ETag / Range / SPA Fallback)
  |
  +---> [API proxy] matches api_proxy_path ---> Backend API upstream
```

## Component Responsibilities

| Component | Responsibilities | Design Reference |
| --- | --- | --- |
| **OpenFlare Server** | Admin UI/API, state persistence, `sites.v1` and `tunnel-routing.v1` compilation, multi-provider DNS pointing and failover, SmartDNS snapshots and quality matrix, Pages bundle storage, access log aggregation, Uptime Kuma sync, and login PoW | [OpenFlare Edge Architecture Design](./pingora-edge-design.md) / [SmartDNS System Architecture Design](./smartdns-design.md) / [Multi-Provider DNS Pointing & Failover Design](./cloudflare-pointing.md) / [Edge Observability and Traffic Analytics Architecture](./observability-design.md) / [Uptime Kuma Sync Design](./kuma-design.md) / [Login CAPTCHA Design](./login-captcha.md) |
| **OpenFlare Edge** | Executes TLS termination, WAF DAG and OWASP CRS rules, PoW challenges, site auth, TinyUFO caching, Pages serving, origin error pages, and isolated active probing | [OpenFlare Edge Architecture Design](./pingora-edge-design.md) / [WAF Orchestration Rule Design](./waf-orchestration-design.md) / [PoW Protection Design](./pow-design.md) / [Edge Cache Strategy Design](./edge-cache-design.md) / [Pages Static Hosting Design](./pages-design.md) / [Origin Error Page Design](./origin-error-page.md) |
| **OpenFlare DNS** | Standalone `hickory-server` and `zen-engine` authoritative resolver with ECS awareness, sharded RRL, offline snapshot failover, and quality-weighted selection | [SmartDNS System Architecture Design](./smartdns-design.md) / [Network Intelligence and Link Quality Architecture](./network-quality-design.md) |
| **OpenFlare Relay** | Public Rust relay embedding `rathole` server and Relay Gateway (`:8443`) for L4/L7 forwarding, Edge colocation, draining, and hot updates | [Tunnel & Intranet Penetration Design](./tunnel-design.md) |
| **OpenFlared** | Private Rust connector performing parallel relay RTT probing, Closest-K zone diversity, Make-Before-Break failover, and CIDR SSRF checks | [Tunnel & Intranet Penetration Design](./tunnel-design.md) |

## Control & Data Plane Synchronization

### 1. Snapshot Compilation & Sync Flow

```text
Admin saves config -> compiles sites.v1 / tunnel-routing.v1 / DNS snapshot
                            |
          +-----------------+-----------------+
          | (WebSocket wake or heartbeat)     |
          v                                   v
   [OpenFlare Edge]                [OpenFlare DNS / Relay / OpenFlared]
Pull manifests by SHA-256         Pull authoritative snapshot & tunnel topology
Verify and swap in memory         Reload resolver & rathole sessions in-process
```

### 2. Core Data Tables

| Domain | Tables |
| --- | --- |
| **Sites & routing** | `of_zones`, `of_zone_domains`, `of_proxy_routes`, `of_origins`, `of_config_versions`, `of_edge_snapshot_state`, `of_tls_certificates` |
| **Multi-provider DNS** | `of_dns_accounts`, `of_cf_connections`, `of_cf_pointing_groups`, `of_cf_pointing_members` |
| **SmartDNS & quality** | `of_dns_instances`, `of_dns_policies`, `of_network_quality_matrix`, `of_as_topology_edges`, `of_traceroute_records`, `of_probe_targets`, `of_probe_target_states` |
| **Pages hosting** | `of_pages_projects`, `of_pages_project_sources`, `of_pages_project_source_runtime`, `of_pages_deployments`, `of_pages_deployment_files` |
| **Nodes & tunnels** | `of_nodes`, `of_node_tags`, `of_node_tag_bindings`, `of_node_system_profiles`, `of_node_cert_deliveries`, `of_apply_logs`, `of_tunnel_relay_pools`, `of_tunnel_relays`, `of_tunnels`, `of_tunnel_connectors`, `of_tunnel_attachments`, `of_tunnel_services` |
| **WAF & security** | `of_waf_rule_groups`, `of_waf_ip_groups`, `of_waf_rule_group_bindings`, `of_waf_request_whitelist`, `of_attack_groups`, `of_pow_policies` |
| **Observability (`logstore`)** | `of_node_access_logs`, `of_node_metric_snapshots`, `of_node_ping_records`, `of_network_quality_samples`, `of_dns_metric_snapshots`, `w_user_access_logs` |

## Key Design Decisions

| Decision | Rationale |
| --- | --- |
| Versioned full snapshots | Provides consistent boundaries for checksum validation, atomic in-memory swaps, and cold-start recovery |
| Agent pull over SSH push | Avoids Server SSH access to nodes and converges reliably across WebSockets and HTTP heartbeats |
| Separation of Zone domains and route policies | Zones govern root domains and certificate bindings while routes share site policies across hostnames |
| Unified DNS provider adapters | `of_dns_accounts` powers ACME DNS-01, SmartDNS delegation, and multi-provider DNS pointing |
| Embedded `rathole` tunnels | Replaces external `frp` wrappers with native Noise/WS/TCP fallback, Closest-K failover, and CIDR SSRF guards |
| Decoupled runtime & control DB | Edge and DNS nodes serve from local/memory snapshots even during control-plane outages |
| Access logs as traffic truth | Disallows local edge traffic pre-aggregation so dashboard, node, and Zone analytics share identical server semantics |
