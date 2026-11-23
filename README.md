<div align="center">

<h1>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/assets/brand/openflare-horizontal-white.svg">
    <img src="./docs/assets/brand/openflare-horizontal-black.svg" alt="OpenFlare" width="320">
  </picture>
</h1>

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
| HTTP/2 throughput, full WAF, 16 connections × 24 streams | **1,158 requests/s** (3 workers) | **718 requests/s** (1 worker) |
| HTTP/2 throughput, WAF off, 16 connections × 24 streams | **2,990 requests/s** (3 workers) | **2,169 requests/s** (1 worker) |
| Peak retest, WAF off, 22-second sample | **3,948 requests/s** (6 workers, 32 × 24) | **2,273 requests/s** (2 workers, 16 × 24) |

US-A connected directly to the isolated Edge IPs without Cloudflare. The tests served a 923 B Pages file with rate limiting disabled and stored access logs on the in-memory filesystem `tmpfs`. The new asynchronous log writer has not been benchmarked on these nodes.

[Benchmark report: test configuration, WAF comparison, and thread scaling.](./docs/en/reference/pingora-benchmark.md)

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

Install OpenFlare Edge on a Linux `amd64` or `arm64` edge server with systemd. Allow inbound TCP ports `80` and `443`. In the dashboard, copy the Discovery Token from **System Settings → OpenFlare → Discovery Token & Deployment**. Replace the server URL and token in this command:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh | sudo bash -s -- --server-url 'https://openflare.example.com' --token-type discovery --token '<DISCOVERY_TOKEN>'
```

The installer downloads the latest stable Edge release and starts the `openflare-edge` systemd service. To attach an existing node, use its node token and set `--token-type node`.

Update an installed Edge node to the latest stable release:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash
```

Switch an installed Edge node to the most recently published prerelease:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash -s -- canary
```

---

## 📚 Documentation

Official Documentation: **[https://openflare.cc](https://openflare.cc)**

* [Quick Start](https://openflare.cc/guide/quick-start)
* [Deployment Guide](https://openflare.cc/deployment/deployment)
* [Configuration Reference](https://openflare.cc/reference/configuration)
* [System Design](https://openflare.cc/design/)

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
