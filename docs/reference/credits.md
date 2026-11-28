# 引用与致谢

OpenFlare 基于众多开源项目与公开数据集构建，在此向所有上游作者、维护者与贡献者致谢。

## 核心组件与衍生来源

| 项目 | 许可证 | 说明 |
| :--- | :--- | :--- |
| **[Pingora](https://github.com/cloudflare/pingora) & [Pingap](https://github.com/vicanso/pingap)** | Apache-2.0 | 边缘数据面（`openflare-edge`）异步网络代理框架与内存/磁盘两级缓存实现 |
| **[Rathole](https://github.com/rathole-org/rathole)** | Apache-2.0 | 内嵌于 `openflare-relay` 与 `openflared` 的 Rust 内网穿透隧道引擎 |
| **[Hickory DNS](https://github.com/hickory-dns/hickory-dns)** | MIT / Apache-2.0 | `openflare-dns` 的权威 DNS 服务器与协议编解码引擎（`hickory-server` / `hickory-proto`） |
| **[ZEN Engine (GoRules Zen)](https://github.com/gorules/zen)** | MIT | 内嵌于 `openflare-dns` 的 Rust 业务规则引擎，用于评估 SmartDNS JDM 调度策略 |
| **[Trippy](https://github.com/fujiapple852/trippy)** | Apache-2.0 | `openflare-probe` 的多协议（ICMP/UDP/TCP）Traceroute 探测与原始套接字权限隔离核心（`trippy-core` / `trippy-privilege`） |
| **[Komari](https://github.com/komari-monitor/komari)** | MIT | 节点主机监控指标、延迟探测（`PingTask` / `PingRecord`）与远程任务模型参考 |
| **[komari-monitor-rs](https://github.com/GenshinMinecraft/komari-monitor-rs)** | WTFPL | 纯 Rust 探针库 `openflare-probe`（`shared/probe`）的衍生基础 |
| **[OWASP Core Rule Set (CRS)](https://github.com/coreruleset/coreruleset) & [zentinel-modsec](https://github.com/zentinelproxy/zentinel-modsec)** | Apache-2.0 | 边缘 WAF 内置 CRS v4.30.0 规则子集与纯 Rust ModSecurity 规则引擎 |
| **[Anubis](https://github.com/TecharoHQ/anubis)** | MIT | 网关无感 PoW（Proof of Work）防 CC 人机挑战机制参考 |

## IP、ASN 与网络情报数据

| 数据源 | 许可证 / 使用条款 | 说明 |
| :--- | :--- | :--- |
| **[MaxMind GeoLite2](https://www.maxmind.com)**（镜像：[FyraLabs/geolite2](https://github.com/FyraLabs/geolite2) · [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip)） | CC BY-SA 4.0 / MaxMind GeoLite2 EULA | 提供 `Country`、`City`、`ASN` MMDB 数据库，用于 WAF 地域匹配、SmartDNS 定位、日志与 ASN 查询 |
| **[sapics/ip-location-db](https://github.com/sapics/ip-location-db)** | PDDL-1.0 | 可选开放 GeoIP 与 ASN MMDB 数据源（`geo-whois-asn-country-mmdb`、`asn-mmdb`） |
| **[bgp.tools](https://bgp.tools/)** | Attribution Required | 全球 BGP 路由表（`table.jsonl`）、ASN 名录（`asns.csv`）与特性标签（`tags/*.txt`） |
| **[CAIDA AS Relationships](https://publicdata.caida.org/datasets/as-relationships/serial-2/)** | CAIDA Public AUA（Citation Required） | 可选运行时加载的 `serial-2` AS 互联关系数据集，用于上游拓扑分析（不随发行包或镜像分发） |

## 主要依赖库与工具链

- **边缘与节点（Rust）**：[Tokio](https://tokio.rs/) (MIT)、[Hyper](https://hyper.rs/) (MIT)、[Rustls](https://github.com/rustls/rustls) (Apache-2.0 / ISC / MIT)、[Reqwest](https://github.com/seanmonstar/reqwest) (MIT / Apache-2.0)、[sysinfo](https://github.com/GuillaumeGomez/sysinfo) (MIT)、[Moka](https://github.com/moka-rs/moka) (MIT / Apache-2.0)、[HdrHistogram](https://github.com/HdrHistogram/HdrHistogram_rust) (MIT / Apache-2.0)、[maxminddb](https://github.com/oschwald/maxminddb-rust) (ISC)、[Clap](https://github.com/clap-rs/clap) (MIT / Apache-2.0)
- **后端（Go）**：[Gin](https://github.com/gin-gonic/gin) (MIT)、[GORM](https://gorm.io/) (MIT)、[Goose](https://github.com/pressly/goose) (Apache-2.0)、[Asynq](https://github.com/hibiken/asynq) (MIT)、[go-redis](https://github.com/redis/go-redis) (BSD-2-Clause)、[Ristretto](https://github.com/dgraph-io/ristretto) & [Otter](https://github.com/maypok86/otter) (Apache-2.0)、[miekg/dns](https://github.com/miekg/dns) (BSD-3-Clause)、[Lego](https://github.com/go-acme/lego) (MIT)、[Ory Fosite](https://github.com/ory/fosite) & [go-oidc](https://github.com/coreos/go-oidc) (Apache-2.0)、[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (MIT)、[OpenTelemetry Go](https://opentelemetry.io/) (Apache-2.0)、[Cobra](https://github.com/spf13/cobra) & [Viper](https://github.com/spf13/viper) (Apache-2.0)、[MaxMind DB Reader](https://github.com/oschwald/maxminddb-golang) (ISC)、[Expr](https://github.com/expr-lang/expr) (MIT)、[Zap](https://github.com/uber-go/zap) (MIT)、[ClickHouse Go](https://github.com/ClickHouse/clickhouse-go) (MIT)、[Gorilla WebSocket](https://github.com/gorilla/websocket) (BSD-2-Clause)、[Swaggo](https://github.com/swaggo/swag) (Apache-2.0)
- **前端与文档**：[Next.js](https://nextjs.org/) & [React](https://react.dev/) (MIT)、[Tailwind CSS](https://tailwindcss.com/) (MIT)、[shadcn/ui](https://ui.shadcn.com/) & [Radix UI](https://www.radix-ui.com/) (MIT)、[XYFlow](https://reactflow.dev/) (MIT)、[TanStack Query & Table](https://tanstack.com/) (MIT)、[Apache ECharts](https://echarts.apache.org/) (Apache-2.0) & [Recharts](https://recharts.org/) (MIT)、[CodeMirror](https://codemirror.net/) (MIT)、[Lucide](https://lucide.dev/) (ISC)、[next-intl](https://next-intl-docs.vercel.app/) (MIT)、[Fumadocs](https://fumadocs.vercel.app/) (MIT)、[Mermaid](https://mermaid.js.org/) (MIT)、[Biome](https://biomejs.dev/) (MIT / Apache-2.0)、[Vitest](https://vitest.dev/) (MIT)

## 署名与许可声明

- OpenFlare 源码采用 [Apache License 2.0](https://github.com/Rain-kl/OpenFlare/blob/main/LICENSE) 开源；衍生代码版权与修改说明保留于根目录 [NOTICE](https://github.com/Rain-kl/OpenFlare/blob/main/NOTICE)。
- 本产品包含由 MaxMind 创建的 GeoLite2 数据（[https://www.maxmind.com](https://www.maxmind.com)）。
- 网络情报与路由分析功能使用 [bgp.tools](https://bgp.tools/) 数据，并支持在运行时加载 [CAIDA AS Relationships](https://www.caida.org/catalog/datasets/as-relationships/) 数据集。
- 所有第三方软件与数据集均保留其原有版权与许可证条款；如发现署名或授权信息遗漏，请通过 [GitHub Issues](https://github.com/Rain-kl/OpenFlare/issues) 联系更正。
