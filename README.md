<div align="center">

# OpenFlare

**Next-Gen Distributed Reverse Proxy & Edge Security Gateway for Pingora**

**[English](./README.md) | [简体中文](./README.zh-CN.md)**

<p align="center">
  <a href="https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/LICENSE">
    <img src="https://img.shields.io/github/license/Rain-kl/OpenFlare?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/releases/latest">
    <img src="https://img.shields.io/github/v/release/Rain-kl/OpenFlare?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/Rain-kl/OpenFlare/pkgs/container/openflare">
    <img src="https://img.shields.io/badge/GHCR-ghcr.io%2Frain--kl%2Fopenflare-brightgreen" alt="ghcr">
  </a>
</p>

</div>

---

## 📖 What is OpenFlare?

**OpenFlare** is a modern, open-source **distributed reverse proxy and edge security gateway platform** designed for multi-server infrastructures and edge clusters.

Rather than a simple single-host admin panel, OpenFlare provides Pingora edge nodes with **centralized cluster orchestration, dynamic zero-reload sync, edge WAF protection, and secure in-network tunneling**:

* ⚡ **Zero-Reload Dynamic Orchestration**: Centralized control plane manages any number of edge nodes. Synchronize rule diffs in milliseconds with zero Nginx reload disruptions.
* 🛡️ **Built-in Edge Security & Anti-CC (Turnstile Alternative)**: A WAF rule engine, MaxMind GeoIP regional ACLs, and **client-side cryptographic Proof of Work (PoW) challenges** help mitigate abusive traffic at the edge.
* 🚇 **Zero-Trust Tunnels (Cloudflare Tunnel Alternative)**: Securely publish private homelab and internal web services to edge proxies via lightweight `openflared` clients without public IPs or open inbound ports.
* 📦 **Pages Edge Hosting**: Host static sites and SPAs pulled directly from GitHub Releases or uploaded bundles, distributed across edge nodes with instant rollbacks and fallback routing.
* 🔒 **Automated Certificates & SSO**: Automatic multi-domain Let's Encrypt certificates via ACME and enterprise single sign-on with GitHub OAuth and standard OIDC.

---

## 🌐 Flexible Deployment: Standalone or Behind Cloudflare

OpenFlare is designed to operate seamlessly either as an independent edge gateway or in harmony with Cloudflare and public CDNs:

### 1. Standalone Edge Gateway (Private CDN)
Ideal for developers managing multi-region VPS clusters or homelabs:
* Deploy your own high-availability CDN and edge proxy cluster with 100% data and privacy ownership.
* Protect origin sites with built-in Turnstile-like PoW human challenges, dynamic WAF, and automated SSL certs without expensive enterprise CDN security plans.

### 2. Behind Cloudflare (Hardened Origin Shield)
Functions as an intelligent **origin shield and routing gateway** behind Cloudflare:
* **Leaked Real IP Protection**: If attackers bypass Cloudflare and directly hit your origin's public IP address, OpenFlare's edge WAF and PoW challenges will immediately intercept and block the attack.
* **Full Private Log & Metric Retention**: Overcomes Cloudflare Free tier limitations by retaining comprehensive, searchable request logs and operational metrics locally.
* **Unified In-Network Routing**: Ingest incoming Cloudflare traffic and dynamically route it across internal backends or private homelab services through secure tunnels.

---

## 📊 Comparison: Traditional Nginx vs NPM vs OpenFlare

| Feature | Traditional Nginx (`nginx.conf`) | Nginx Proxy Manager (NPM) | **OpenFlare** |
| :--- | :--- | :--- | :--- |
| **Cluster Architecture** | Manual per-host editing via SSH | Single node only; no cluster sync | **Native Distributed Control Plane + Edge Agents** |
| **Configuration Sync** | Requires `nginx -s reload` (connection churn) | Reloads on every change | **Shared memory dynamic sync (Zero-Reload)** |
| **Anti-CC / Bot Protection** | Basic `limit_req` rate limiting only | Not supported | **Built-in Turnstile-like PoW Client Challenge** |
| **In-Network Tunnels** | Requires separate third-party tools (frp) | Not supported | **Built-in OpenFlared Tunnels (Zero inbound ports)** |
| **WAF & Threat Defense** | Requires manual ModSecurity compilation | Basic exploit rules only | **Dynamic WAF, GeoIP, IP list checksum sync** |
| **Static Pages Hosting** | Manual directory copying and setup | Static file proxying only | **Automated GitHub Release deployment & rollbacks** |
| **Works with Cloudflare** | Manual `set_real_ip_from` list maintenance | Complex proxy header tuning | **Native real IP restoration, ideal origin shield** |

---

## 🏗️ Architecture

```mermaid
flowchart TD
    subgraph Visitors ["Public Visitors & Clients"]
        User["Public Users / Browsers"]
        CF["Cloudflare CDN (Optional Edge Layer)"]
    end

    subgraph EdgePlane ["Edge Data Plane (Pingora Nodes)"]
        Edge1["Edge Node A (Pingora)"]
        Edge2["Edge Node B (Pingora)"]
    end

    subgraph ControlPlane ["Control Plane"]
        Server["OpenFlare Server (Web Dashboard)"]
        DB[("PostgreSQL + Redis")]
    end

    subgraph Backend ["Upstream & Internal Services"]
        Origin["Public Origin Web Server"]
        subgraph PrivateNet ["Private Network / Homelab"]
            NAS["Internal Apps / NAS"]
            Client["OpenFlared Tunnel Client"]
        end
    end

    User -->|Direct HTTPS Request| Edge1
    User -->|Proxied via Cloudflare| CF
    CF -->|Secure Upstream Traffic| Edge2

    Server <-->|Dynamic Sync / Metrics| Edge1
    Server <-->|Dynamic Sync / Metrics| Edge2
    Server --- DB

    Edge1 -->|Reverse Proxy| Origin
    Edge2 -->|Reverse Proxy| Origin

    Client <-->|Encrypted Tunnel| Edge1
    NAS --- Client
```

---

## Pingora Performance Benchmarks

**November 20, 2026 · OpenFlare Edge `0.1.8` · Load generator: US-A**

| Measured metric | JP-B | US-C |
| --- | ---: | ---: |
| Pingora workers (automatic sizing) | 3 | 1 |
| Direct public-IP RTT | 115.7 ms | 1.0 ms |
| Direct HTTPS keep-alive P50 / P95 (40 requests/s) | 117.7 / 120.2 ms | 3.8 / 6.9 ms |
| Direct HTTP/2 download (30 MiB, median end-to-end rate) | **111 Mbps** | **775 Mbps** |
| HTTP/2 successful throughput: full WAF / IP only / WAF off | **420 / 486 / 488 requests/s** | **274 / 256 / 283 requests/s** |
| Edge CPU at 160 requests/s: full WAF / IP only / WAF off | **32.2% / 12.1% / 10.2%** | **25.7% / 9.4% / 8.7%** |

The public-IP tests bypassed Cloudflare. The isolated WAF comparison used a 923 B Pages file, 8 connections × 16 HTTP/2 streams, and no per-IP request-rate limit. CPU percentages are relative to one vCPU.

[Benchmark report: performance policy, per-run results, and methodology.](./docs/en/reference/pingora-benchmark.md)

---

## 🖥️ UI Preview

### Dashboard Overview
![OpenFlare Dashboard Overview](./docs/assets/readme/dashboard-overview.png)

### Edge WAF & Access Rules
![OpenFlare WAF Protection](./docs/assets/readme/waf.png)

### Domain & Traffic Analytics
![OpenFlare Domain Overview](./docs/assets/readme/domain_overview.png)

---

## ⚡ Quick Start

### 1. Launch OpenFlare Server

Deploy the control plane using `docker-compose`:

```bash
# Download environment template
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
```

```yaml
services:
  openflare:
    image: ghcr.io/rain-kl/openflare:latest
    restart: unless-stopped
    env_file: .env
    environment:
      TZ: ${TZ:-Asia/Shanghai}
    ports:
      - "3000:3000"
    volumes:
      - openflare_uploads:/app/uploads
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-openflare}
      POSTGRES_USER: ${DB_USERNAME:-openflare}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-replace-with-strong-password}
    volumes:
      - openflare_postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USERNAME:-openflare} -d ${DB_NAME:-openflare}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: valkey/valkey:8.0-alpine
    restart: unless-stopped
    command: ["valkey-server", "--appendonly", "yes"]
    volumes:
      - openflare_redis_data:/data
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s

volumes:
  openflare_uploads:
  openflare_postgres_data:
  openflare_redis_data:
```

* **Dashboard URL**: `http://localhost:3000`
* **Default Credentials**: `admin` / `12345678`

> [!WARNING]
> Change the default password `12345678` immediately upon first login.  
> The BETA version is in active development and testing; avoid using it in mission-critical production environments without regular backups.

### 2. Connect Edge Agent Node

Run the Agent container on any edge server (ensure ports 80/443 are open):

```bash
docker pull ghcr.io/rain-kl/openflare-agent:latest
docker run -d --name openflare-agent --restart unless-stopped \
  -p 80:80 -p 443:443/tcp -p 443:443/udp \
  -v openflare-agent-pages:/data/var/lib/openflare/pages \
  -e OPENFLARE_SERVER_URL=http://<YOUR_SERVER_IP>:3000 \
  -e OPENFLARE_AGENT_TOKEN=<YOUR_AGENT_TOKEN> \
  ghcr.io/rain-kl/openflare-agent:latest
```
*(Copy the node token directly from Node Management in the dashboard)*

---

## 📚 Documentation

Official Documentation: **[https://openflare.fyrn.link](https://openflare.fyrn.link)**

* [Quick Start](https://openflare.fyrn.link/guide/quick-start)
* [Deployment Guide](https://openflare.fyrn.link/deployment/deployment)
* [Configuration Reference](https://openflare.fyrn.link/reference/configuration)
* [System Design](https://openflare.fyrn.link/design/)

<details>
<summary><b>🔍 Minimum & Recommended Hardware Specifications</b></summary>

| Component | Minimum Requirements | Recommended Requirements | Note |
| :--- | :--- | :--- | :--- |
| **Server Control Plane** | 1 CPU / 2 GB RAM / 20 GB Disk | 2 CPU / 4 GB RAM / 50 GB+ Disk | Scale disk based on access log retention |
| **Agent Data Plane** | 1 CPU / 512 MB RAM / 2 GB Disk | 2 CPU / 2 GB RAM / 10 GB+ Disk | Scale based on concurrent connections & WAF load |
| **Relay Node** | 1 CPU / 1 GB RAM / 5 GB Disk | 2 CPU / 2 GB RAM / 20 GB Disk | Mainly bound by bandwidth and CPU network throughput |
| **OpenFlared Client** | 1 CPU / 256 MB RAM / 1 GB Disk | 1 CPU / 512 MB RAM / 5 GB Disk | Extremely lightweight client |

</details>

---

## ⚖️ Trademark Disclaimer

OpenFlare is an independent, community-driven open-source project. It is not affiliated with, endorsed by, or sponsored by Cloudflare, Inc. Cloudflare and related trademarks are the property of their respective owners.

---

## 📄 License

This project is licensed under the [Apache License 2.0](./LICENSE).

## ⭐ Star History

<a href="https://www.star-history.com/?repos=Rain-kl%2FOpenFlare&type=date&legend=bottom-right">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=Rain-kl/OpenFlare&type=date&legend=top-left" />
 </picture>
</a>
