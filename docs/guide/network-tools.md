# 网络工具与网络质量情报

OpenFlare「网络工具」子系统提供控制平面与多边缘节点协同的网络诊断能力，并在底层持续构建 **边缘节点 × 访客 ASN** 的全网路由与延迟质量矩阵，为后续智能流量调度与多地域容灾提供真实测量依据。

---

## 核心架构与隔离设计

1. **多源网络情报库（NetIntel）**
   - 内置并自动热加载 GeoLite2 / sapics GeoIP 与 ASN 数据库，以及 `bgp.tools` 全球 BGP 路由表（`bgptools-table.jsonl.gz`）、ASN 组织名录（`bgptools-asns.csv`）与特性标签库（`bgptools-asn-tags.txt`）。
   - 控制平面启动时自动从 `netintel_data_dir`（默认 `data/netintel`）加载数据集（缺省时回落至内嵌 `GeoLite2-Country.mmdb`），并通过定期异步任务 `of_netintel_sync` 原子下载更新与 Redis 广播热重载。
   - **许可与合规说明**：镜像构建阶段仅预置可再分发数据库；**CAIDA AS Relationships** 数据集因受 CAIDA AUA（Acceptable Use Agreement）分发约束，永不打入发行镜像，仅在管理员显式将系统配置 `netintel_caida_enabled` 设为 `true` 后于运行期增量拉取。

2. **被动 TCP 测量与精确分布统计**
   - OpenFlare Edge（Linux）在每个下游连接进入访问日志流程且有界队列尚有余量时，轻量读取一次内核 `TCP_INFO`（`tcpi_rtt`、`tcpi_min_rtt`、`tcpi_rttvar`、`tcpi_total_retrans`、`tcpi_segs_out`）与 TLS 握手耗时，**零阻塞且绝不影响反向代理转发主线程性能**。
   - 服务端在访问日志入库路径无锁识别访客 IP 的 `client_asn`、国家、细分地区（GeoIP Subdivision）及地址族（`ipv4`/`ipv6`），按 `(node_id, client_asn, client_country, client_region, ip_family, source)` 写入有界 1 分钟内存分桶（最多 8192 桶，80% 水位线异步触发提前刷盘）。
   - 分桶内放弃有偏均分位均值，改用 **64 桶对数直方图（`LogHistogram64`，覆盖 `[0.5ms, 16384ms]`，相对误差 `< 8%`）** 与 **64 位 Bottom-k 去重前缀草图（`PrefixSketch`）** 统计独立网段覆盖率，同时使用球面三角学合成加权地理质心与按时间戳排序的指数移动平均（`rtt_ewma_ms`），并写入包含 25 列的 `of_network_quality_samples`。

3. **两级精确质量矩阵与 5 级延迟预测引擎（Phase 1.5）**
   - 定期任务 `of_netquality_rollup` 将实测样本独立聚合被动 TCP（`tcp_*`）、主动 ICMP/TCPing（`probe_*`）与路由追踪（`trace_*`）指标，生成两级实测矩阵行（`of_network_quality_matrix`）：
     - `region` 级：对高地域跨度白名单国家（`CN, US, RU, CA, BR, AU, IN, JP, DE, GB`）保留省/州粒度（如 `CN-GD`、`US-CA`）；
     - `country` 级：在国家级保留统一回退基准。
     - Rollup 仅落盘实测数据（`data_source='measured'`），并递增快照代数 `RollupGeneration` 失效旧预测缓存。
   - 5 级按需延迟预测器（`Predictor`）支持单次或批量边缘节点排序，按可信度由高到低依次执行：`exact_region`（精确省/州实测） → `country_rollup`（国家级实测） → `inferred_upstream`（基于多维网络相似度、同组线路过滤与非负几何残差缩并的同节点相似上游迁移） → `similar_node`（冷启动节点严格限定同城 `<= 50km` 且同 `network_group` 兄弟节点加权映射） → `geo_prior`（球面地理传输基线）。

4. **受控主动探测靶点池与双向 SSRF 防护**
   - 取消早期猜测试探前缀 `.1` 的盲测机制，所有后台主动探测（`of_netquality_probe`）均严格调度管理表 `of_probe_targets` 中已启用的公网靶点，并按 `(node_id, target_id)` 维度在 `of_probe_target_states` 中记录连续失败次数、最近报错与指数退避时间（`15m` 起步，上限 `24h`）。
   - 调度优先级综合 **流量热度 × 预测低置信度/候选离散度 × 数据陈旧度 × 非最优备用节点探索奖励**，并受全局、单节点及单靶点发包/并发预算控制。超时或不可达仅更新退避状态，绝不写入 `0ms` 伪样本；标记为 `anycast_status='confirmed'` 的锚点严格隔离于 `of_as_topology_edges` 与跨区域矩阵泛化之外。
   - **双向 SSRF 防线**：控制平面创建/编辑靶点以及 Rust 边缘探测工作进程（`shared/probe`）在执行 Ping/TCPing/HTTP/Traceroute/DNS 解析前均独立校验解析结果与目标地址，强制拒绝回环、RFC1918 私网、链路本地（`169.254.0.0/16` / `fe80::/10`）、CGNAT（`100.64.0.0/10`）、保留基准网段（`198.18.0.0/15`）、组播及未指定地址，HTTP 探测同时禁用跨跳重定向（`Policy::none`）。

5. **独立子进程探测运行时（`probe-worker`）**
   - 多节点主动探测（批量 ICMP/TCPing/HTTP Ping、基于 `trippy-core` 的多轮 Traceroute、带 EDNS0 ECS 的 DNS 探测）全部由 Edge 托管并隔离在独立操作系统子进程 `openflare-edge probe-worker` 中执行。
   - Linux 下子进程启用 `PR_SET_PDEATHSIG(SIGTERM)`、禁用 core dump（`RLIMIT_CORE = 0`）并降低调度优先级（`nice +10`）；主控或子进程任一单次探测异常或崩溃均会被监督器（`ProbeSupervisor`）隔离与退避重启，不会波及边缘反向代理与控制面主进程。
   - 权限授予说明（`CAP_NET_RAW`）：systemd 节点通过 `install-openflare-edge.sh` 与 `update-openflare-edge.sh` 在服务单元的 `AmbientCapabilities` 及 `CapabilityBoundingSet` 中授予 `CAP_NET_RAW`；Docker 节点通过镜像内置 `cap_net_raw+p` 并在启动参数附加 `--cap-add=NET_RAW` 获得权限。若早期 systemd 节点通过控制台自更新升级而来，可重新运行 `update-openflare-edge.sh`，或手动在 `/etc/systemd/system/openflare-edge.service` 的 `AmbientCapabilities` 与 `CapabilityBoundingSet` 中补充 `CAP_NET_RAW` 后执行 `systemctl daemon-reload && systemctl restart openflare-edge`。

---

## 控制台功能页面

在左侧导航栏 **「网络工具」** 分组中提供五个独立功能页，同时可在 **「节点管理」** 编辑弹窗中配置边缘节点的线路分组（`network_group`，例如 `cn2-gia`、`cmi`、`bgp`）以隔离差异化网络线路：

| 页面路径 | 功能说明 |
| :--- | :--- |
| `/network-tools/ping` | **多点 Ping / TCPing**：向指定或全部在线边缘节点并发下发批量 ICMP、TCPing 或 HTTP 探测，汇总各节点的最小/平均/P95 延迟、抖动、丢包率及逐包明细。目标地址输入支持自动纠正：ICMP / TCPing 模式下粘贴完整 URL（含协议、路径、查询参数或凭据）时将自动仅保留主机地址并发起探测，URL 中显式声明的端口会自动填入端口输入框。 |
| `/network-tools/traceroute` | **路由追踪（Traceroute）**：支持 ICMP、UDP 与 TCP 模式的多轮次路径探测，逐跳自动标注地理位置、ASN、组织名称与 `bgp.tools` 标签，并记录历史与观测到的 AS 跳边（`of_as_topology_edges`）。 |
| `/network-tools/asn` | **ASN 情报查询**：输入任意公网 IP、域名或 AS 编号（如 `AS13335`），查看地理位置、最长匹配 BGP 前缀、IPv4/IPv6 宣告前缀分页、**上游路由接线拓扑图**（展示 `Origins → Upstreams → Tier 1 ISPs` 分层上游与骨干连接，支持下钻跳转与导出 SVG）、AS 邻接关系与接入 5 级预测器的多边缘节点路径与质量对比。 |
| `/network-tools/dns` | **DNS 诊断查询**：支持控制平面直查与多边缘节点并发探测，可指定解析器与 EDNS0 客户端子网（ECS）CIDR 模拟不同访客区域的权威返回，并自动富化应答 IP 的 ASN 与地区归属。 |
| `/network-tools/quality` | **网络质量矩阵与探测靶点**：在 **「质量融合矩阵」** 页签中支持按国家/地区、聚合层级（`country`/`region`）、IP 地址族及实测来源筛选质量矩阵；在独立的 **「质量模拟预测」** 页签中提供客户端网络质量模拟预测面板（调用 `POST /api/v1/d/network-tools/quality-matrix/predict` 实时演练 5 级预测器打分与校准误差）；提供 **「探测靶点」** 管理页签管理公网靶点、Anycast 隔离状态及各节点退避明细；同时支持在 **「情报数据库状态」** 页签查看数据集状态与一键触发同步（Data powered by [bgp.tools](https://bgp.tools)）。 |

---

## 相关系统配置

可在 **「管理 → 系统配置」** 中按需调整以下参数：

- `netintel_data_dir`：网络情报数据库本地目录（默认 `data/netintel`）。
- `netintel_geoip_source`：GeoIP/ASN 数据库上游源（`fyralabs` 或 `sapics`）。
- `netintel_bgptools_user_agent`：请求 `bgp.tools` 公开接口时携带的 `User-Agent` 标识。
- `netintel_caida_enabled`：是否启用运行期 CAIDA AS Relationships 数据同步（默认 `false`）。
- `netintel_auto_probe_enabled`：是否启用后台定时任务 `of_netquality_probe` 从在线边缘节点自动探测活跃 ASN 路径（默认 `true`）。
