# Open Source Acknowledgments and Dependencies

The creation and ongoing evolution of OpenFlare rely deeply on the outstanding contributions of the global open-source community. From the underlying Rust high-performance network proxy engine, secure multi-protocol tunneling, and authoritative DNS rule evaluation to WAF rulesets, host telemetry probes, IP/ASN intelligence datasets, and a modern frontend interface, the platform integrates dozens of exceptional open-source software libraries, rulesets, and engineering frameworks.

We extend our sincere respect and gratitude to the authors, maintainers, and contributors of all referenced projects. This page catalogs the core open-source projects, derivative code origins, datasets, and licensing information utilized throughout OpenFlare.

---

## Core Systems and Derivative Origins

### 1. Pingora & Pingap
- **Role in Project**: Edge network data plane engine and in-memory cache layer
- **License**: Apache-2.0
- **Overview**: Cloudflare's high-performance Rust asynchronous multi-threaded network service framework, combined with Pingap cache extensions. Powers OpenFlare Edge nodes (`openflare-edge`) with core proxy capabilities, dynamic TLS termination, WAF filtering pipelines, client Proof-of-Work (PoW) challenges, two-tier cache eviction, and hitless configuration snapshot synchronization.
- **Repositories**: [cloudflare/pingora](https://github.com/cloudflare/pingora) · [vicanso/pingap](https://github.com/vicanso/pingap)

### 2. Wavelet
- **Role in Project**: Derivative source origin and control plane architectural foundation
- **License**: Apache-2.0
- **Overview**: OpenFlare includes software derived from Wavelet, including its core control plane patterns, permission models, and foundational designs. Original copyright notices and modification statements are retained in the project root's [NOTICE](https://github.com/Rain-kl/OpenFlare/blob/main/NOTICE) file.
- **Copyright Statement**: `Copyright 2025 Arctel.net. Licensed under the Apache License, Version 2.0.`

### 3. Rathole
- **Role in Project**: Intranet penetration and encrypted tunnel transport engine
- **License**: Apache-2.0
- **Overview**: A reverse proxy for NAT traversal written in Rust. OpenFlare embeds `rathole` directly in-process inside both the tunnel relay (`openflare-relay`) and connector (`openflared`), supporting Noise, WebSocket, and TCP transports with hot-reloadable configuration for L4 TCP/UDP forwarding and L7 gateway proxying.
- **Repository**: [rathole-org/rathole](https://github.com/rathole-org/rathole)

### 4. Gin Web Framework
- **Role in Project**: Server HTTP routing and middleware pipeline
- **License**: MIT
- **Overview**: High-performance Go HTTP web framework serving as the primary API router for the OpenFlare Server control plane, powering RESTful dashboard requests, multi-node authentication handshakes, and event distribution.
- **Repository**: [gin-gonic/gin](https://github.com/gin-gonic/gin)

### 5. ZEN Engine (GoRules Zen)
- **Role in Project**: SmartDNS JDM scheduling policy and expression evaluation engine
- **License**: MIT
- **Overview**: An open-source Rust business rules engine supporting JSON Decision Model (JDM) graphs and expression evaluation. OpenFlare's standalone authoritative DNS server (`openflare-dns`) embeds `zen-engine` to evaluate visual scheduling policies against client location, ASN, ECS subnet, and node routing tags.
- **Repository**: [gorules/zen](https://github.com/gorules/zen)

### 6. Komari
- **Role in Project**: Node observability, latency probing, and remote task schema design
- **License**: MIT
- **Overview**: A lightweight self-hosted server monitoring and management project. OpenFlare aligns its node hardware/OS profiles, real-time telemetry snapshots, latency probe records (`PingTask` / `PingRecord`), and remote command execution models (`Task` / `TaskResult`) with Komari's monitoring schema.
- **Repository**: [komari-monitor/komari](https://github.com/komari-monitor/komari)

### 7. komari-monitor-rs
- **Role in Project**: Rust host probe, latency check, and remote execution foundation
- **License**: WTFPL
- **Overview**: A Rust monitoring agent implementation for the Komari ecosystem. OpenFlare's reusable pure-Rust probe crate `openflare-probe` (`shared/probe`) is derived from and modernized based on `komari-monitor-rs`, supplying host telemetry collection, latency probing, and bounded remote execution for node runtimes.
- **Repository**: [GenshinMinecraft/komari-monitor-rs](https://github.com/GenshinMinecraft/komari-monitor-rs)

### 8. OWASP Core Rule Set (CRS) & zentinel-modsec
- **Role in Project**: Edge WAF attack-protection ruleset and ModSecurity evaluation engine
- **License**: Apache-2.0
- **Overview**: `openflare-edge` bundles an unmodified subset of OWASP CRS v4.30.0 (`edge/vendor/owasp-crs`) and executes request initialization and attack-category rules in pure Rust via `zentinel-modsec`.
- **Repositories**: [coreruleset/coreruleset](https://github.com/coreruleset/coreruleset) · [zentinelproxy/zentinel-modsec](https://github.com/zentinelproxy/zentinel-modsec)

---

## IP, ASN, and Network Intelligence Databases

| Dataset | License | Purpose & Integration Scope | Source |
| :--- | :--- | :--- | :--- |
| **MaxMind GeoLite2 (FyraLabs / Loyalsoldier mirrors)** | CC BY-SA 4.0 / MaxMind GeoLite2 EULA | Provides `GeoLite2-Country`, `GeoLite2-City`, and `GeoLite2-ASN` MMDB databases (`Loyalsoldier/geoip` as the embedded fallback Country DB) for WAF geo-matching, SmartDNS geolocation, access-log enrichment, and ASN lookup | [FyraLabs/geolite2](https://github.com/FyraLabs/geolite2) · [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip) |
| **sapics / ip-location-db** | PDDL-1.0 | Alternative open GeoIP Country and ASN MMDB source (`@ip-location-db/geo-whois-asn-country-mmdb` and `@ip-location-db/asn-mmdb`) selectable for control-plane network intelligence sync | [sapics/ip-location-db](https://github.com/sapics/ip-location-db) |
| **bgp.tools** | bgp.tools (Attribution Required) | Global BGP routing table (`table.jsonl`), ASN organization directory (`asns.csv`), and ASN characteristic tags (`tags/*.txt`) powering prefix-to-ASN resolution and network diagnostics | [bgp.tools](https://bgp.tools/) |
| **CAIDA AS Relationships** | CAIDA Public AUA (Runtime-Only, Citation Required) | Optional runtime AS-relationship dataset (`serial-2`) loaded by `pkg/netintel` for upstream ISP topology and path analysis (never redistributed in binaries or container images) | [CAIDA AS Relationships](https://publicdata.caida.org/datasets/as-relationships/serial-2/) |

---

## Key Backend Infrastructure and Libraries

The OpenFlare Server control plane (Go stack) is built upon production-proven open-source libraries:

| Project | License | Purpose & Integration Scope |
| :--- | :--- | :--- |
| **[GORM](https://gorm.io/)** | MIT | Comprehensive ORM supporting dual-dialect switching (PostgreSQL / SQLite) and read-write segregation |
| **[Goose](https://github.com/pressly/goose)** | Apache-2.0 | Declarative database migration tool managing DDL evolution and initial configuration seeds across databases |
| **[Asynq](https://github.com/hibiken/asynq)** | MIT | Redis-backed distributed task queue and background job scheduler for certificate renewal and batch operations |
| **[Lego](https://github.com/go-acme/lego)** | MIT | ACME client library automating Let's Encrypt multi-domain SSL/TLS certificate requests, validation, and renewal |
| **[OpenTelemetry Go SDK](https://opentelemetry.io/)** | Apache-2.0 | Distributed tracing and telemetry instrumentation exporting spans and metrics via OTLP gRPC/HTTP |
| **[Cobra](https://github.com/spf13/cobra) & [Viper](https://github.com/spf13/viper)** | Apache-2.0 | Modern CLI framework and hierarchical configuration loader supporting environment variable overrides |
| **[MaxMind DB Reader](https://github.com/oschwald/maxminddb-golang)** | ISC | High-speed GeoLite2 binary database reader providing country and regional ACL filtering for edge WAF rules |
| **[Expr](https://github.com/expr-lang/expr)** | MIT | Fast expression evaluation engine powering complex multi-attribute condition matching in WAF policies |
| **[Zap](https://github.com/uber-go/zap)** | MIT | Uber's zero-allocation structured logging library ensuring fast diagnostics under high concurrency |
| **[ClickHouse Go](https://github.com/ClickHouse/clickhouse-go)** | MIT | Official ClickHouse native driver for high-throughput access log persistence and security analytics |
| **[Swaggo](https://github.com/swaggo/swag)** | Apache-2.0 | Automated OpenAPI 2.0 / Swagger documentation generator extracted directly from Go source comments |

---

## Frontend and User Interface Stack

The OpenFlare Web Console (`frontend/`) is built on React 19 and Next.js, featuring flat, shadowless design conventions:

| Project | License | Purpose & Integration Scope |
| :--- | :--- | :--- |
| **[Next.js](https://nextjs.org/) & [React](https://react.dev/)** | MIT | Modern App Router SSR and static HTML generation framework (SSG / Standalone export) |
| **[Tailwind CSS](https://tailwindcss.com/)** | MIT | Utility-first CSS framework establishing unified layout tokens and light/dark theme transitions |
| **[shadcn/ui](https://ui.shadcn.com/) & [Radix UI](https://www.radix-ui.com/)** | MIT | Accessible, composable UI component primitives and interaction controls |
| **[XYFlow (React Flow)](https://reactflow.dev/)** | MIT | Interactive node-based diagramming library for visual DAG orchestration of WAF filtering rules |
| **[TanStack Query & Table](https://tanstack.com/)** | MIT | Asynchronous state management, caching, and high-performance virtualized data tables |
| **[Apache ECharts](https://echarts.apache.org/) & [Recharts](https://recharts.org/)** | Apache-2.0 / MIT | Charting engines for edge bandwidth, HTTP status codes, and security mitigation time series |
| **[Lucide Icons](https://lucide.dev/)** | ISC | Consistent, restrained vector icons used throughout the console and documentation |
| **[next-intl](https://next-intl-docs.vercel.app/)** | MIT | Internationalization routing and localized message catalogs (English / Simplified Chinese / Russian) |

---

## Documentation and Tooling

| Project | License | Purpose & Integration Scope |
| :--- | :--- | :--- |
| **[Fumadocs](https://fumadocs.vercel.app/)** | MIT | Modern Next.js and MDX-based documentation generation framework |
| **[Biome](https://biomejs.dev/)** | MIT / Apache-2.0 | High-performance Rust-native formatter and linter for web assets |
| **[Vitest](https://vitest.dev/)** | MIT | Fast unit testing runner ensuring reliable behavior across business and utility modules |

---

## Licensing and Intellectual Property Compliance

1. **OpenFlare License**: The OpenFlare project source code is distributed under the [Apache License 2.0](https://github.com/Rain-kl/OpenFlare/blob/main/LICENSE).
2. **Third-Party Rights**: All third-party libraries, frameworks, and tools retain their original copyrights and licenses. Use, modification, and redistribution are governed by their respective terms.
3. **Attribution and Modifications**: For derivative software and origin disclosures, please refer to the [NOTICE](https://github.com/Rain-kl/OpenFlare/blob/main/NOTICE) file at the root of the repository.
4. **Corrections and Inquiries**: We take intellectual property and open-source license compliance seriously. If you identify any missing or inaccurate copyright or licensing statements, please submit feedback via [GitHub Issues](https://github.com/Rain-kl/OpenFlare/issues) for prompt review and resolution.

We extend our deepest appreciation to every developer, designer, and maintainer contributing to the open-source ecosystem.
