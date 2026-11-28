# 网络情报与链路质量感知架构

OpenFlare 通过控制面多源网络情报（`NetIntel`）、边缘被动 `TCP_INFO` 采集、受控主动探测（`openflare-edge probe-worker`），以及多时间窗口自适应质量矩阵与 5 级延迟预测器，为诊断工具与 SmartDNS 提供可量化的节点—网络链路质量评估能力。

---

## 1. 多源网络情报底座 (`internal/apps/openflare/netintel`)

控制面在进程内维护只读内存索引 `pkg/netintel.Manager`（`Lock-free` 读路径），由异步任务 `of_netintel_sync` 定时拉取更新并原子切换；启用 Redis 时通过 `netintel:reloaded` 频道向其他控制面副本广播重载信号。

| 数据源 | 本地文件 | 默认来源与同步策略 | 许可 / 用途 |
| :--- | :--- | :--- | :--- |
| **GeoIP / ASN** | `GeoLite2-{Country,City,ASN}.mmdb` 或 `sapics-{country,asn}.mmdb` | 默认 FyraLabs GeoLite2 镜像（支持切换 `sapics`）；1 小时内未过期跳过（`force=true` 可强制刷新）；不可用时回退内嵌 Country MMDB | 国家、ISO 3166-2 行政区、经纬度、精度半径与归属 ASN |
| **BGP 路由与标签** | `bgptools-asns.csv`、`bgptools-tags.txt`、`bgptools-asn-tags.txt`、`bgptools-table.jsonl.gz` | 从 `bgp.tools` 下载全网路由前缀与标签索引，单文件下载上限 `512 MiB`（索引/标签文本 `32 MiB`） | IP 最长前缀匹配、ASN 名称/分类标签及前缀列表 |
| **AS 商业关系（可选）** | `caida-as-rel2.txt.bz2` | 仅当 `netintel_caida_enabled = true` 时从 CAIDA serial-2 下载并校验 bzip2 帧完整性，默认新鲜窗口 12 小时 | AS 邻居拓扑、Tier-1 可达性与上游路径推断 |

同步完成后自动为缺失 `asn` / `asn_org` 的边缘节点执行回填（每批最多 500 条），并持久化 `manifest.json`。

CAIDA 关系图以连续邻接表存储。每个邻接项保存对端 ASN 和关系编码，名称、国家和标签在查询时从 ASN 元数据补齐。数据加载保留邻接项的文件顺序，路径搜索优先遍历实测 traceroute 边。

---

## 2. 被动质量采集 (`openflare-edge` → `netquality`)

边缘网关在不引入额外主动发包开销的前提下，从真实客户端连接提取传输层往返时延与重传指标：

```text
访客 TCP 连接完成请求
  └─ openflare-edge proxy（access_logs 队列未满时读取内核 TCP_INFO）
       └─ 随心跳上报 tcp_rtt_us / tcp_min_rtt_us / tcp_rtt_var_us / tcp_retrans / tcp_segs_out / tls_handshake_ms
            └─ Server passive_collector（每 60s 刷新或达高水位提前刷盘）
                 ├─ 无锁查询 NetIntel：client_asn、client_country、client_region、ip_family、坐标与 GeoSource
                 └─ 按 (node_id, client_asn, client_country, client_region, ip_family) 分桶聚合
                      ├─ LogHistogram64（64 桶对数直方图，覆盖 [0.5 ms, 10000 ms]，谐波-几何代表值）
                      ├─ PrefixSketch（IPv4 /24、IPv6 /48 的 FNV-64 bottom-64 去重覆盖素描）
                      └─ 球面加权质心（SphericalCentroid）与最小 RTT
                           └─ 批量写入 logstore（of_network_quality_samples）
```

* **采样边界**：控制面内存分桶上限为 `maxPassiveBuckets = 8192`（达 80% 即 `6553` 桶提前触发 flush），丢弃超过 `60s`（`60,000,000 µs`）的异常样本，避免冷门或随机源分布撑爆控制面内存。
* **地址族严格隔离**：`ipv4`、`ipv6` 与 `unknown` 各自独立统计，禁止通过固定比例跨地址族推算延迟。

---

## 3. 自适应多窗口质量矩阵与 5 级预测器

异步任务 `of_netquality_rollup` 定期扫描最近 24 小时内的 `of_network_quality_samples`（含主动与被动来源，单次最多读取 `100,000` 条）以及 `of_traceroute_records`（最多 `5,000` 条），合并生成两级聚合行（`country` 与白名单国家的 `region` 级下钻）写入 `of_network_quality_matrix`。

### 3.1 多时间窗口自适应状态机 (`adaptive_state.go`)

针对每个 `(node_id, client_asn, client_country, client_region, ip_family, aggregation_level)` 单元，系统同时跟踪三条时间窗口并决定当前有效延迟（`effective_rtt_ms`）与健康状态（`quality_state`）：

1. **基准层（24 小时 / 6 小时半衰期）**与 **EWMA（2 小时半衰期）**，合并 `tcp_passive`、`tcp_active`、`icmp_active` 与 `traceroute` 信号；
2. **瞬时窗口（`10m`）**与**短窗口（`45m`，20 分钟半衰期）**：
   * 样本量满足门限且 `p50` 较基线上升 `>= 1.35x` 且增量 `>= 15 ms`（10 分钟窗，短窗为 `1.30x` / `>= 12 ms`），或重传/丢包率 `>= 5%` 且高于基线 `>= 3%` 时，状态立即切换为 `degraded`，并立刻改用受损窗口的真实 P50 作为 `effective_rtt_ms`；
   * 恢复期需在后续至少连续 `2` 个确认窗口内恢复至参考延迟 `<= 1.25x + 15 ms` 且无丢包劣化：第 1 个良好窗口转入 `recovering`（更新 `observed_rtt_ms`，保守维持原有受损 `effective_rtt_ms`），第 2 个窗口确认后恢复 `healthy`；
   * 若最近观测距今超过 `60` 分钟，标记为 `stale`；此前若处于受损态，保留受损 `effective_rtt_ms`，防止断流导致调度误将故障链路视为低延迟。

### 3.2 动态 P95 尾延迟惩罚与探索优先级

* **动态 P95 权重**：当样本数 `< 10` 时尾延迟权重为 `0`；达到门限后随样本量（尺度 `20`）与独立 `/24` 或 `/48` 前缀数（`PrefixSketch`）平滑爬升至上限 `0.35`，过滤单客户抖动引起的误判。
* **探索优先级 (`explore_priority`)**：结合样本陈旧度、预测误差不确定度与节点相对最优延迟比值，驱动受控主动探测定期检验冷门或陈旧路径，避免静态黑盒锁死。

### 3.3 五级延迟预测器 (`predictor.go`)

`Predictor` 维护 5 分钟 TTL、最多 `4096` 条的进程内缓存（矩阵提交递增 `CurrentRollupGeneration()` 时全量失效）；`lat` / `lon` 按 `0.2°` 量化分桶，`confirmed` Anycast 目标不跨地域复用 L3 缓存。查询按以下优先级依次回落：

| 级别 | 方法 (`PredictionMethod`) | 匹配规则与适用场景 |
| :--- | :--- | :--- |
| **L1** | `exact_region` | 精确命中 `(node_id, client_asn, client_country, client_region, ip_family)` |
| **L2** | `country_rollup` | 回落同国家聚合行，置信度乘以 `0.90` 衰减因子 |
| **L3** | `inferred_upstream` | 在同一节点上筛选相同 `client_country` 或通过 BGP/AS 上游相似度加权残差迁移（要求总权重 `>= 0.12`，收缩因子 `1.20`，置信度封顶 `0.72`；输出加权 IQR 作为 `CandidateDispersionMs`） |
| **L4** | `similar_node` | 当目标节点在矩阵中样本极少（`<= 2` 行）时，参考同 `network_group` 或距目标节点 `<= 50 km`（或同 ASN）兄弟节点的测量结果，置信度乘 `0.65` |
| **L5** | `geo_prior` | 纯物理光纤几何先验（`1.35 * (2d / 200) + 8 ms`；缺乏有效坐标时回退 `120 ms`、置信度 `0.05`） |

`ComputeQualityScore(res)` 按 `effective_rtt_ms + 0.25 * CandidateDispersionMs` 与丢包惩罚合成 `[1.0, 100.0]` 分值（而不改写物理可读的 `RTTMs` 字段）。

### 3.4 SmartDNS 接入契约与热路径无 SQL 约束

* **防腐层边界（`internal/apps/openflare/qualityapi`）**：SmartDNS 不直接读取 `netquality` 或 `pkg/netintel` 内部表结构，仅通过 `qualityapi.Provider` 消费归一化预测结果 `PredictionResult`：
  * `RTTMs`、`ObservedRTTMs`、`PredictedRTTMs`：有效延迟与观测/基线对照；
  * `RTTP95Ms` / `P95Weight`：尾延迟估计及动态权重（`[0, 0.35]`）；
  * `CandidateDispersionMs` / `ErrorMs`：L3/L4 候选加权 IQR 及经验 P80 留一校准误差（未校准时明确为 `nil`）；
  * `QualityState`、`ExplorePriority`、`MeasurementSource`、`SourceTier`（`measured_region` / `measured_country` / `inferred` / `geo_prior`）、`GeoBaseRTTMs` / `GeoBaseKnown`（缺坐标回落 `120 ms`）与 `AnycastIsolated`（确认 Anycast 时置 `true` 阻断跨区域迁移推断）。
* **热路径无 SQL**：控制面内嵌 DNS 引擎通过带 `60s` 内存缓存的 `QualityProvider`（随 `CurrentRollupGeneration()` 刷新）查询；独立 `openflare-dns` 实例则直接索引快照内携带的 `CompactQualitySnapshot` 与 `CompactNodePrior` 完成 O(1) 内存查找，禁止在单次 DNS 查询内执行 SQL。

---

## 4. 受控主动探测与隔离工作进程 (`shared/probe`)

主动诊断（控制台即时 Ping / Traceroute / DNS 查询）与后台巡检任务 `of_netquality_probe` 复用同一套主动探测与安全校验边界。

### 4.1 自动巡检预算与退避 (`of_netquality_probe`)

由系统配置 `netintel_auto_probe_enabled` 控制（默认开启），通过 WebSocket 下发至在线且未隔离的 `edge_node` 节点：

* **目标账本与退避**：目标列表存于 `of_probe_targets`（默认 seed 一批公共锚点），各 `(node_id, target_id)` 退避状态存于 `of_probe_target_states`。最小冷却期 `20` 分钟；连续失败后采用指数退避（基准 `15` 分钟，上限 `24` 小时）。
* **严格发包配额**：每轮控制面全局最多派发 `24` 个探测任务（单节点最多 `4` 个目标、包成本上限 `120`；单目标最多分配 `4` 台节点）。其中单轮 ICMP Ping 默认 `4` 包，Traceroute 默认最大 `24` 跳 × `2` 包（估算成本 `48`）。

### 4.2 双向 SSRF 防护 (`internal/apps/openflare/nettools/safety.go` & `shared/probe/src/safety.rs`)

控制面在接收请求时校验目标，边缘探测执行前再次解析并二次核验目标地址，阻止以下网络目标（除非诊断接口明确允许且符合规则）：

* 回环地址、私有网段（RFC 1918 / ULA）、链路本地、CGNAT（`100.64.0.0/10`）、基准测试保留段（`198.18.0.0/15`）、组播、未指定地址以及 IPv4 映射的 IPv6 地址；
* 内网域名后缀（`localhost`、`.localhost`、`.local`、`.internal`、`.localdomain`、`.home.arpa`）。

### 4.3 `openflare-edge probe-worker` 隔离执行器 (`shared/probe`)

`openflare-edge` 主代理进程自身不长期持有原始套接字循环，而是通过 `ProbeSupervisor`（任务队列上限 `64`、单任务超时 `50s`、异常退出按 `1s..30s` 指数退避重启）拉起独立的子命令进程 `openflare-edge probe-worker`（默认空闲 `300s` 自动退出）：

* **内核与权限隔离**：Linux 下 `probe-worker` 启动即设置 `PR_SET_PDEATHSIG(SIGTERM)`（父进程崩溃时内核自动清理孤儿）、`PR_SET_DUMPABLE=0`、`RLIMIT_CORE=0` 与 `nice +10` 低调度优先级，仅在执行 ICMP/Traceroute 会话时调用 `trippy_privilege` 启用 `CAP_NET_RAW`，随后完成原始套接字收发；
* **本地发包预算守卫**：即使上游指令异常突发，`probe-worker` 仍强制执行 60 秒滑动窗口上限——全局最多 `180` 次调度 / `2,400` 个探测包，单个目标最多 `16` 次调度 / `600` 个探测包（跟踪槽位上限 `2048`），防止边缘节点被滥用为探测放大源。
