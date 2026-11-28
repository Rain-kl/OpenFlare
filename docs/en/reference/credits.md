# Acknowledgments & Third-Party Notices

OpenFlare builds on open-source software and public network datasets. We thank all upstream authors, maintainers, and contributors.

## Core Engines & Derived Software

| Project | License | Role in OpenFlare |
| :--- | :--- | :--- |
| **[Pingora](https://github.com/cloudflare/pingora) & [Pingap](https://github.com/vicanso/pingap)** | Apache-2.0 | Edge data-plane proxy framework and multi-tier cache implementation for `openflare-edge` |
| **[Rathole](https://github.com/rathole-org/rathole)** | Apache-2.0 | Embedded Rust NAT-traversal tunnel engine powering `openflare-relay` and `openflared` |
| **[Hickory DNS](https://github.com/hickory-dns/hickory-dns)** | MIT / Apache-2.0 | Authoritative DNS server and protocol engine (`hickory-server` / `hickory-proto`) powering `openflare-dns` |
| **[ZEN Engine (GoRules Zen)](https://github.com/gorules/zen)** | MIT | Embedded Rust business rules engine evaluating SmartDNS JDM scheduling policies in `openflare-dns` |
| **[Trippy](https://github.com/fujiapple852/trippy)** | Apache-2.0 | Multi-protocol (ICMP/UDP/TCP) traceroute probing and raw-socket privilege isolation (`trippy-core` / `trippy-privilege`) for `openflare-probe` |
| **[Komari](https://github.com/komari-monitor/komari)** | MIT | Reference schema for host telemetry, latency probing (`PingTask` / `PingRecord`), and remote command tasks (`Task` / `TaskResult`) |
| **[komari-monitor-rs](https://github.com/GenshinMinecraft/komari-monitor-rs)** | WTFPL | Upstream codebase from which `openflare-probe` (`shared/probe`) was derived and modernized |
| **[OWASP Core Rule Set (CRS)](https://github.com/coreruleset/coreruleset) & [zentinel-modsec](https://github.com/zentinelproxy/zentinel-modsec)** | Apache-2.0 | Bundled CRS v4.30.0 attack-protection subset and pure-Rust ModSecurity rule engine for Edge WAF |
| **[Anubis](https://github.com/TecharoHQ/anubis)** | MIT | Inspiration for client Proof-of-Work (PoW) challenge mitigation |

## IP, ASN & Network Intelligence Data

| Dataset | License / Terms | Role in OpenFlare |
| :--- | :--- | :--- |
| **[MaxMind GeoLite2](https://www.maxmind.com)** (mirrors: [FyraLabs/geolite2](https://github.com/FyraLabs/geolite2) · [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip)) | CC BY-SA 4.0 / MaxMind GeoLite2 EULA | `Country`, `City`, and `ASN` MMDB databases for WAF geo-matching, SmartDNS geolocation, access logs, and ASN lookup |
| **[sapics/ip-location-db](https://github.com/sapics/ip-location-db)** | PDDL-1.0 | Alternative open GeoIP Country and ASN MMDB source (`geo-whois-asn-country-mmdb`, `asn-mmdb`) |
| **[bgp.tools](https://bgp.tools/)** | Attribution Required | Global BGP routing table (`table.jsonl`), ASN directory (`asns.csv`), and ASN tags (`tags/*.txt`) |
| **[CAIDA AS Relationships](https://publicdata.caida.org/datasets/as-relationships/serial-2/)** | CAIDA Public AUA (Citation Required) | Optional runtime-loaded `serial-2` AS relationship dataset for upstream topology analysis (never redistributed in releases or images) |

## Key Libraries & Tooling

- **Edge & Node (Rust)**: [Tokio](https://tokio.rs/) (MIT), [Hyper](https://hyper.rs/) (MIT), [Rustls](https://github.com/rustls/rustls) (Apache-2.0 / ISC / MIT), [Reqwest](https://github.com/seanmonstar/reqwest) (MIT / Apache-2.0), [sysinfo](https://github.com/GuillaumeGomez/sysinfo) (MIT), [Moka](https://github.com/moka-rs/moka) (MIT / Apache-2.0), [HdrHistogram](https://github.com/HdrHistogram/HdrHistogram_rust) (MIT / Apache-2.0), [maxminddb](https://github.com/oschwald/maxminddb-rust) (ISC), [Clap](https://github.com/clap-rs/clap) (MIT / Apache-2.0)
- **Backend (Go)**: [Gin](https://github.com/gin-gonic/gin) (MIT), [GORM](https://gorm.io/) (MIT), [Goose](https://github.com/pressly/goose) (Apache-2.0), [Asynq](https://github.com/hibiken/asynq) (MIT), [go-redis](https://github.com/redis/go-redis) (BSD-2-Clause), [Ristretto](https://github.com/dgraph-io/ristretto) & [Otter](https://github.com/maypok86/otter) (Apache-2.0), [miekg/dns](https://github.com/miekg/dns) (BSD-3-Clause), [Lego](https://github.com/go-acme/lego) (MIT), [Ory Fosite](https://github.com/ory/fosite) & [go-oidc](https://github.com/coreos/go-oidc) (Apache-2.0), [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (MIT), [OpenTelemetry Go](https://opentelemetry.io/) (Apache-2.0), [Cobra](https://github.com/spf13/cobra) & [Viper](https://github.com/spf13/viper) (Apache-2.0), [MaxMind DB Reader](https://github.com/oschwald/maxminddb-golang) (ISC), [Expr](https://github.com/expr-lang/expr) (MIT), [Zap](https://github.com/uber-go/zap) (MIT), [ClickHouse Go](https://github.com/ClickHouse/clickhouse-go) (MIT), [Gorilla WebSocket](https://github.com/gorilla/websocket) (BSD-2-Clause), [Swaggo](https://github.com/swaggo/swag) (Apache-2.0)
- **Frontend & Docs**: [Next.js](https://nextjs.org/) & [React](https://react.dev/) (MIT), [Tailwind CSS](https://tailwindcss.com/) (MIT), [shadcn/ui](https://ui.shadcn.com/) & [Radix UI](https://www.radix-ui.com/) (MIT), [XYFlow](https://reactflow.dev/) (MIT), [TanStack Query & Table](https://tanstack.com/) (MIT), [Apache ECharts](https://echarts.apache.org/) (Apache-2.0) & [Recharts](https://recharts.org/) (MIT), [CodeMirror](https://codemirror.net/) (MIT), [Lucide](https://lucide.dev/) (ISC), [next-intl](https://next-intl-docs.vercel.app/) (MIT), [Fumadocs](https://fumadocs.vercel.app/) (MIT), [Mermaid](https://mermaid.js.org/) (MIT), [Biome](https://biomejs.dev/) (MIT / Apache-2.0), [Vitest](https://vitest.dev/) (MIT)

## Attribution & License Notices

- OpenFlare is licensed under the [Apache License 2.0](https://github.com/Rain-kl/OpenFlare/blob/main/LICENSE). Derivative notices are retained in [NOTICE](https://github.com/Rain-kl/OpenFlare/blob/main/NOTICE).
- This product includes GeoLite2 data created by MaxMind, available from [https://www.maxmind.com](https://www.maxmind.com).
- Network intelligence features use data from [bgp.tools](https://bgp.tools/) and optionally load [CAIDA AS Relationships](https://www.caida.org/catalog/datasets/as-relationships/) at runtime.
- All third-party projects and datasets remain subject to their respective copyrights and licenses. Report any missing or inaccurate attribution via [GitHub Issues](https://github.com/Rain-kl/OpenFlare/issues).
