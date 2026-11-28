# 引用与致谢

OpenFlare 在设计与实现过程中借鉴了众多开源项目的优秀理念、架构设计和技术实现。以下是 OpenFlare 在核心底层引擎、安全防护机制、可观测探针、IP/ASN 数据源以及前后端系统框架等方面所引用的关键开源项目与公开数据集，在此对这些项目及其社区表示感谢。

---

## 核心系统与衍生来源

### 1. Pingora 与 Pingap
*   **项目定位**：Cloudflare 开源的 Rust 异步多线程高性能网络代理框架及 Pingap 缓存实现。
*   **在 OpenFlare 中的作用**：作为全局数据面（Data Plane）的边缘网关核心引擎。边缘节点（`openflare-edge`）基于 Pingora 构建，负责接收全部公网 Web 流量，执行动态 TLS 握手与证书管理、WAF 安全规则比对、PoW 客户端人机验证、两级边缘缓存与反向代理转发。
*   **项目链接**：[cloudflare/pingora (GitHub)](https://github.com/cloudflare/pingora) 与 [vicanso/pingap (GitHub)](https://github.com/vicanso/pingap)

---

### 2. Rathole
*   **项目定位**：基于 Rust 编写的 NAT 内网穿透反向代理（Apache-2.0）。
*   **在 OpenFlare 中的作用**：作为内网穿透子系统的底层隧道引擎。中继端 `openflare-relay` 与连接器 `openflared` 以库形式在进程内直接嵌入 `rathole`，支持 Noise、WebSocket 与 TCP 传输及配置热加载，承载 L4 TCP/UDP 转发与 L7 网关代理流量。
*   **项目链接**：[rathole-org/rathole (GitHub)](https://github.com/rathole-org/rathole)

---

### 3. ZEN Engine（GoRules Zen）
*   **项目定位**：基于 Rust 编写的开源业务规则引擎，支持 JDM（JSON Decision Model）决策图与表达式求值（MIT）。
*   **在 OpenFlare 中的作用**：作为 SmartDNS 权威解析子系统的策略评估引擎。独立权威服务端 `openflare-dns` 嵌入 `zen-engine` 执行控制台编排生成的 JDM 调度策略，结合客户端位置、ASN、ECS 子网与节点标签权重完成解析决策。
*   **项目链接**：[gorules/zen (GitHub)](https://github.com/gorules/zen)

---

### 4. Komari
*   **项目定位**：轻量级自托管服务器监控与管理方案（MIT）。
*   **在 OpenFlare 中的作用**：OpenFlare 借鉴了 Komari 的主机监控指标、延迟探测（`PingTask` / `PingRecord`）与远程命令执行（`Task` / `TaskResult`）模型及日志表结构设计，用于统一节点可观测数据采集与展示。
*   **项目链接**：[komari-monitor/komari (GitHub)](https://github.com/komari-monitor/komari)

---

### 5. komari-monitor-rs
*   **项目定位**：使用 Rust 编写的 Komari 监控采集端实现（WTFPL）。
*   **在 OpenFlare 中的作用**：OpenFlare 的纯 Rust 探针库 `openflare-probe`（`shared/probe`）衍生自 `komari-monitor-rs` 并进行了重构，为 `openflare-edge`、`openflare-relay` 等节点运行时提供主机资源采集、延迟探测与受控远程执行能力。
*   **项目链接**：[GenshinMinecraft/komari-monitor-rs (GitHub)](https://github.com/GenshinMinecraft/komari-monitor-rs)

---

### 6. OWASP Core Rule Set（CRS）与 zentinel-modsec
*   **项目定位**：OWASP 开源 Web 应用防火墙核心规则集（v4.30.0，Apache-2.0）及纯 Rust ModSecurity 规则引擎（Apache-2.0）。
*   **在 OpenFlare 中的作用**：边缘节点 `openflare-edge` 内置 OWASP CRS v4.30.0 攻击防护规则子集（`edge/vendor/owasp-crs`），并通过 `zentinel-modsec` 在 Rust 数据面中执行请求初始化与各类攻击特征检测。
*   **项目链接**：[coreruleset/coreruleset (GitHub)](https://github.com/coreruleset/coreruleset) 与 [zentinelproxy/zentinel-modsec (GitHub)](https://github.com/zentinelproxy/zentinel-modsec)

---

### 7. Anubis（PoW 方案）
*   **项目定位**：基于工作量证明（Proof of Work）的轻量级人机验证防护方案。
*   **在 OpenFlare 中的作用**：为网关 WAF 提供了核心的**无感防 CC 人机挑战**能力。

---

### 8. gin-template
*   **项目定位**：基于 Go Gin 与前端构建的现代化全栈开发脚手架模板。
*   **在 OpenFlare 中的作用**：为 OpenFlare 控制面（Server）提供了规范、统一的前后端系统架构雏形。

---

## IP、ASN 与网络情报数据库

| 数据源 | 许可证 | 在 OpenFlare 中的作用 | 项目 / 数据地址 |
| :--- | :--- | :--- | :--- |
| **MaxMind GeoLite2（FyraLabs / Loyalsoldier 镜像）** | CC BY-SA 4.0 / MaxMind GeoLite2 EULA | 提供 `GeoLite2-Country`、`GeoLite2-City` 与 `GeoLite2-ASN` MMDB 数据库（`Loyalsoldier/geoip` 作为默认内置回退国家库），用于 WAF 地域拦截、SmartDNS 位置识别、访问日志解析及 ASN 归属查询 | [FyraLabs/geolite2](https://github.com/FyraLabs/geolite2) · [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip) |
| **sapics / ip-location-db** | PDDL-1.0 | 可选的开放 GeoIP 与 ASN MMDB 数据源（`@ip-location-db/geo-whois-asn-country-mmdb` 与 `@ip-location-db/asn-mmdb`），用于控制面网络情报切换与补充解析 | [sapics/ip-location-db](https://github.com/sapics/ip-location-db) |
| **bgp.tools** | bgp.tools（Attribution Required） | 提供全球 BGP 路由表（`table.jsonl`）、ASN 组织名录（`asns.csv`）与特性标签（`tags/*.txt`），支撑网络工具中的前缀归属查询与网络情报分析 | [bgp.tools](https://bgp.tools/) |
| **CAIDA AS Relationships** | CAIDA Public AUA（仅运行时加载，需引用署名） | 可选运行时 AS 互联关系数据集（`serial-2`），用于控制面分析自治系统上游路径与网络拓扑（不随安装包或容器镜像分发） | [CAIDA AS Relationships](https://publicdata.caida.org/datasets/as-relationships/serial-2/) |

---
