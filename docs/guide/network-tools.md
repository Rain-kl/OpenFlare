# 网络工具与网络质量情报

OpenFlare「网络工具」子系统提供控制平面与多边缘节点协同的网络诊断能力，并在底层持续构建 **边缘节点 × 访客 ASN** 的全网路由与延迟质量矩阵，为后续智能流量调度与多地域容灾提供真实测量依据。

---

## 核心架构与隔离设计

1. **多源网络情报库（NetIntel）**
   - 内置并自动热加载 GeoLite2 / sapics GeoIP 与 ASN 数据库，以及 `bgp.tools` 全球 BGP 路由表（`bgptools-table.jsonl.gz`）、ASN 组织名录（`bgptools-asns.csv`）与特性标签库（`bgptools-asn-tags.txt`）。
   - 控制平面启动时自动从 `netintel_data_dir`（默认 `data/netintel`）加载数据集（缺省时回落至内嵌 `GeoLite2-Country.mmdb`），并通过定期异步任务 `of_netintel_sync` 原子下载更新与 Redis 广播热重载。
   - **许可与合规说明**：镜像构建阶段仅预置可再分发数据库；**CAIDA AS Relationships** 数据集因受 CAIDA AUA（Acceptable Use Agreement）分发约束，永不打入发行镜像，仅在管理员显式将系统配置 `netintel_caida_enabled` 设为 `true` 后于运行期增量拉取。

2. **被动 TCP 测量与隔离异步落盘**
   - OpenFlare Edge（Linux）在每个下游连接进入访问日志流程且有界队列尚有余量时，轻量读取一次内核 `TCP_INFO`（`tcpi_rtt`、`tcpi_min_rtt`、`tcpi_rttvar`、`tcpi_total_retrans`、`tcpi_segs_out`）与 TLS 握手耗时，**零阻塞且绝不影响反向代理转发主线程性能**。
   - 服务端在访问日志入库路径无锁识别访客 IP 的 `client_asn`，通过有界 1 分钟内存分桶聚合（最多 4096 桶，每桶蓄水池采样上限 256 条）汇总 P50/P95 RTT 与重传率，每 60 秒批量写入 `of_network_quality_samples`。

3. **独立子进程探测运行时（`probe-worker`）**
   - 多节点主动探测（批量 ICMP/TCPing/HTTP Ping、基于 `trippy-core` 的多轮 Traceroute、带 EDNS0 ECS 的 DNS 探测）全部由 Edge 托管并隔离在独立操作系统子进程 `openflare-edge probe-worker` 中执行。
   - Linux 下子进程启用 `PR_SET_PDEATHSIG(SIGTERM)`、禁用 core dump（`RLIMIT_CORE = 0`）并降低调度优先级（`nice +10`）；主控或子进程任一单次探测异常或崩溃均会被监督器（`ProbeSupervisor`）隔离与退避重启，不会波及边缘反向代理与控制面主进程。
   - 权限授予说明（`CAP_NET_RAW`）：systemd 节点通过 `install-openflare-edge.sh` 与 `update-openflare-edge.sh` 在服务单元的 `AmbientCapabilities` 及 `CapabilityBoundingSet` 中授予 `CAP_NET_RAW`；Docker 节点通过镜像内置 `cap_net_raw+p` 并在启动参数附加 `--cap-add=NET_RAW` 获得权限。若早期 systemd 节点通过控制台自更新升级而来，可重新运行 `update-openflare-edge.sh`，或手动在 `/etc/systemd/system/openflare-edge.service` 的 `AmbientCapabilities` 与 `CapabilityBoundingSet` 中补充 `CAP_NET_RAW` 后执行 `systemctl daemon-reload && systemctl restart openflare-edge`。

---

## 控制台功能页面

在左侧导航栏 **「网络工具」** 分组中提供五个独立功能页：

| 页面路径 | 功能说明 |
| :--- | :--- |
| `/network-tools/ping` | **多点 Ping / TCPing**：向指定或全部在线边缘节点并发下发批量 ICMP、TCPing 或 HTTP 探测，汇总各节点的最小/平均/P95 延迟、抖动、丢包率及逐包明细。目标地址输入支持自动纠正：ICMP / TCPing 模式下粘贴完整 URL（含协议、路径、查询参数或凭据）时将自动仅保留主机地址并发起探测，URL 中显式声明的端口会自动填入端口输入框。 |
| `/network-tools/traceroute` | **路由追踪（Traceroute）**：支持 ICMP、UDP 与 TCP 模式的多轮次路径探测，逐跳自动标注地理位置、ASN、组织名称与 `bgp.tools` 标签，并记录历史与观测到的 AS 跳边（`of_as_topology_edges`）。 |
| `/network-tools/asn` | **ASN 情报查询**：输入任意公网 IP、域名或 AS 编号（如 `AS13335`），查看地理位置、最长匹配 BGP 前缀、IPv4/IPv6 宣告前缀分页、**上游路由接线拓扑图**（展示 `Origins → Upstreams → Tier 1 ISPs` 分层上游与骨干连接，支持下钻跳转与导出 SVG）、AS 邻接关系与多边缘节点到达目标 AS 的路径对比。 |
| `/network-tools/dns` | **DNS 诊断查询**：支持控制平面直查与多边缘节点并发探测，可指定解析器与 EDNS0 客户端子网（ECS）CIDR 模拟不同访客区域的权威返回，并自动富化应答 IP 的 ASN 与地区归属。 |
| `/network-tools/quality` | **网络质量矩阵**：以 6 小时指数衰减窗口融合被动 TCP RTT 与主动路由追踪样本（并在缺失实测链路时按 AS 路径先验生成预测项），并支持查看情报数据库状态与一键触发同步（Data powered by [bgp.tools](https://bgp.tools)）。 |

---

## 相关系统配置

可在 **「管理 → 系统配置」** 中按需调整以下参数：

- `netintel_data_dir`：网络情报数据库本地目录（默认 `data/netintel`）。
- `netintel_geoip_source`：GeoIP/ASN 数据库上游源（`fyralabs` 或 `sapics`）。
- `netintel_bgptools_user_agent`：请求 `bgp.tools` 公开接口时携带的 `User-Agent` 标识。
- `netintel_caida_enabled`：是否启用运行期 CAIDA AS Relationships 数据同步（默认 `false`）。
- `netintel_auto_probe_enabled`：是否启用后台定时任务 `of_netquality_probe` 从在线边缘节点自动探测活跃 ASN 路径（默认 `true`）。
