# Open Source Acknowledgments and Dependencies

The creation and ongoing evolution of OpenFlare rely deeply on the outstanding contributions of the global open-source community. From the underlying Rust high-performance network proxy engine and secure multi-protocol tunneling to server-side distributed task scheduling, enterprise observability pipelines, and a modern frontend interface, the platform integrates dozens of exceptional open-source software libraries and engineering frameworks.

We extend our sincere respect and gratitude to the authors, maintainers, and contributors of all referenced projects. This page catalogs the core open-source projects, derivative code origins, and licensing information utilized throughout OpenFlare.

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

### 3. FRP (Fast Reverse Proxy)
- **Role in Project**: Intranet penetration and secure tunnel transport engine
- **License**: Apache-2.0
- **Overview**: A high-performance reverse proxy application developed by fatedier. OpenFlare's tunnel relay manager (`openflare-relay`) orchestrates and supervises the `frps` core, while the client `openflared` supervises local multiplexed `frpc` tunnel channels to securely expose internal web services without public IPs or inbound open ports.
- **Repository**: [fatedier/frp](https://github.com/fatedier/frp)

### 4. Gin Web Framework
- **Role in Project**: Server HTTP routing and middleware pipeline
- **License**: MIT
- **Overview**: High-performance Go HTTP web framework serving as the primary API router for the OpenFlare Server control plane, powering RESTful dashboard requests, multi-node authentication handshakes, and event distribution.
- **Repository**: [gin-gonic/gin](https://github.com/gin-gonic/gin)

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
