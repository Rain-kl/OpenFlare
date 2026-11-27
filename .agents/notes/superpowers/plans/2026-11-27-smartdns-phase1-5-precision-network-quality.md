# SmartDNS Phase 1.5：网络情报与延迟预测精准化实施计划（定稿）

> **Status:** Approved & Finalized (2026-11-27)
> **Scope:** Phase 1.5A（数据正确性与采集底座）→ Phase 1.5B（延迟预测引擎与离线回测）→ Phase 1.5C（受控主动探测、安全防护与可视化）

---

## 一、背景与要解决的核心问题

Phase 1 已建立多源网络情报（MMDB + `bgp.tools` + 可选 CAIDA）、被动 TCP RTT 采集、隔离式主动探测子进程与质量矩阵框架。但在实际面向全球多地区边缘节点与跨国 ASN 进行调度前，存在六项影响数据正确性、预测准确度与系统安全性的核心缺陷：

1. **维度坍缩（同 ASN 跨国/跨协议混算）**：`of_network_quality_samples` 虽采集了 `client_country`，但 `of_network_quality_matrix` 唯一索引仅为 `(node_id, client_asn)`，且 `rollup_task.go` 的 `matrixPairKey` 与 `latestTracePaths` 丢掉了国家、区域与 `ip_family`，导致跨国 ASN（如 AS174、AS13335）与 IPv4/IPv6 混合计算。
2. **分位数失真与跨协议直接均值**：`rollup_task.go` 对 1 分钟桶的 `rtt_p50_ms`/`rtt_p95_ms` 做加权平均，且将 `tcp_passive` 与 `traceroute` 按 `1.0 : 0.7` 盲目混合；`distinct_prefix_count` 与 `rtt_ewma_ms` 在跨桶聚合时也缺乏可合并数据结构与有序计算。
3. **节点坐标退化为国家质心**：`internal/apps/openflare/geoip/node.go` 使用仅包含 Country 库的 `pkggeoip`，且回填仅检查缺失未区分 `geo_source`，导致未手动锁定的节点长期停留在国家质心坐标。
4. **纯 AS 跳数启发式与预测混入实测矩阵**：`hops * 14 + 12` 与线性跳数推测脱离物理传播距离与实际互联拓扑；每 5 分钟向 `of_network_quality_matrix` 写入 $N_{\text{nodes}} \times N_{\text{networks}}$ 条预测行既导致表膨胀，又污染实测数据并形成反馈闭环。
5. **误差口径混淆与相似度空集漏洞**：候选残差 IQR（离散度）不等同于预测误差；双侧均无上游数据时不可将相似度判为 `1.0`；远端 Tier-1 重合不能视为强同源。
6. **盲 Ping 前缀 `.1` 与单点故障误杀**：`auto_probe_task.go` 从 BGP 表随机取前缀首个可用 IP 探测，易产生大量不可达超时噪声与安全/滥用风险；缺乏按节点隔离的失败退避与双向 SSRF 校验。

---

## 二、已确认的设计决策与核心架构约束

### 1. 待确认事项定案
- **`client_region` 下钻策略**：内置可扩展高离散度区域白名单（默认包含 `CN`、`US`、`RU`、`CA`、`BR`、`AU`、`IN`、`JP`、`DE`、`GB`），采样尽量保留原始 GeoIP `Subdivision`；Rollup 同步维护 `aggregation_level = 'country'`（国家级稳定汇总与回落）和 `aggregation_level = 'region'`（区域精细预测）两层行。
- **探测靶点管理 UI**：在 Phase 1.5C 于 `/network-tools/quality` 同步新增「探测靶点」管理 Tab（查看、增删改、全局启停与按节点维度失败退避状态）。
- **预测参数 $k=1.35$、$b=8\text{ms}$**：暂作为 `netquality` 内部冷启动常量（不暴露系统设置项），待离线回测与真实样本积累后在评估校准逻辑中按历史残差自动拟合。

### 2. 数据与统计架构约束
1. **矩阵唯一键（六元组）**：
   `UNIQUE (node_id, client_asn, client_country, client_region, ip_family, aggregation_level)`
   - 仅对实际观测到的 `ip_family`（`ipv4` / `ipv6`，历史迁移未知为 `unknown`）建行，不预生成空分片。
   - 使用 `aggregation_level`（`'country'` / `'region'`）彻底消除 `client_region = ""` 的“未知区域 vs 全国汇总”语义歧义。区域未知的样本仅参与 `country` 聚合，不污染具体省份行；区域已知样本先形成 `region` 桶，再合并贡献给上级 `country` 行。
2. **64 桶固定对数直方图（`hist_version = 1`）**：
   - 预留 2 个溢出桶：Bucket `0` 为 `< 0.5 ms`，Bucket `63` 为 `> 10000 ms`。
   - 中间 `62` 个正常等比桶覆盖 $[0.5, 10000]\text{ ms}$，相邻桶倍率 $r = (10000 / 0.5)^{1/62} = 20000^{1/62} \approx 1.1732$，区间内几何中点代表值的最大相对近似误差 $\frac{r-1}{r+1} \approx 7.97\%$（远优于 15% 上限）。
   - 单桶计数器 `[64]uint32` 仅 `256 B`（8192 桶仅 `2.00 MiB`），采用紧凑版本化 Base64/varint 或紧凑编码存库，支持加权合并与精确百分位还原。
3. **可合并前缀摘要（`prefix_sketch`）与有序 EWMA**：
   - 采样桶维护有界脱敏前缀哈希集合（上限 64 个 `/24` IPv4 或 `/48` IPv6 HMAC/FNV64 哈希 + `saturated` 饱和标志 + 精确计数兜底）。
   - Rollup 阶段先对各样本的 `prefix_sketch` 求并集再去重计数写入 `distinct_prefix_count`，禁止跨桶直接累加数量。
   - `rtt_ewma_ms` 在 Rollup 内严格按 `captured_at` 升序遍历计算，禁止对分桶 EWMA 直接算术平均。
4. **球面向量质心与地理基准字段分离**：
   - 客户端位置聚合使用三维单位球面向量加权求和再反算经纬度，避免跨国际日期变更线（$179^\circ$ 与 $-179^\circ$）求均值出错；坐标缺失不写入 `(0, 0)`。
   - 明确区分 `geo_base_rtt_ms`（$2d/200$ 球面传播基线）、`geo_accuracy_km`、`geo_source`（`manual` / `city` / `country` / `unknown`）与 `geo_confidence`。
5. **实测矩阵与预测缓存彻底分离**：
   - `of_network_quality_matrix` **只存实测聚合行**（`data_source = 'measured'`），彻底停止 Rollup 向数据库批量物化全量预测行。
   - Rollup 采用“内存完整构建新代际快照 $\to$ 单事务原子更新（Upsert `stats_version = 2` 实测行并清理旧代际 `stats_version < 2` 与遗留 `data_source = 'predicted'` 行）”的零冷启动空窗策略。
   - 按需预测统一由 `Predictor` + 有界 TTL `pkg/cache/ram` 缓存 `prediction_cache` 计算（每次 `RunRollup` 完成后自增代际版本号自动失效）。
6. **纯测量残差与误差/离散度拆解**：
   - 地理残差基于纯测量延迟：$RTT_{\text{extra}} = RTT_{\text{P50}} - RTT_{\text{geo\_base}}$（不掺入抖动/重传惩罚）；允许负残差存在并降低地理权重，不强行截断归零；最终调度评分 `routing_score` 留给调用/调度侧组合。
   - 严格区分 `candidate_dispersion_ms`（候选残差加权 IQR）与 `prediction_error_ms`（仅在离线回测校准映射建立后按条件输出，未校准时保持 `nil`）。

---

## 三、分期实施路线

### Phase 1.5A：数据正确性与采集底座（最高优先级）

> **目标**：修复维度坍缩、分位数失真、节点坐标偏差、前缀计数错误与迁移平滑切换，保证所有落库和聚合数据口径严格正确。

1. **NetIntel 地理增强与节点城市级坐标修复**
   - `pkg/netintel/types.go`、`pkg/netintel/mmdb.go`、`pkg/netintel/manager.go`：
     - `mmdbGeoRecord` 新增 `Continent.Code`、`Subdivisions[0].ISOCode`、`Location.AccuracyRadius`、`Traits.IsAnycast`；
     - `IPLookupResult` 新增 `ContinentCode`、`RegionCode`、`AccuracyRadiusKm`、`GeoSource`（`city` / `country` / `asn_country` / `unknown`）、`AnycastStatus`（`unknown` / `unicast` / `confirmed`）；
     - 新增 `pkg/netintel/geo_math.go`：Haversine `DistanceKm`、`GeoBaseRTTMs`、球面向量质心累加器 `SphericalCentroid`、Anycast 前缀识别辅助与大国 `NormalizeRegionCode(country, subdivision)` 白名单工具。
   - `internal/model/openflare_node.go`、`internal/apps/openflare/geoip/node.go`、`internal/apps/openflare/agent/helpers.go`、`internal/apps/openflare/netintel/sync.go`、`internal/apps/openflare/node/`：
     - `OpenFlareNode` 新增 `GeoSource`（默认 `''`）与可选 `NetworkGroup`（默认 `''`，如 `cn2-gia` / `bgp`）；
     - `!node.GeoManualOverride` 时优先使用 `netintel.LookupIP`（City 库）解析城市经纬度与 `geo_source = "city"`；**允许 City 坐标覆盖历史由 `CountryCentroidByISO` 写入的国家质心坐标**；无 City 坐标时才回落 `pkggeoip` 并标记 `geo_source = "country"`；
     - `netintel.SyncDatasets` 完成后一并检查并回填缺失城市级坐标的未锁定节点；节点详情/编辑接口支持查看 `geo_source` 与设置 `network_group`。

2. **64 桶对数直方图与有界前缀摘要**
   - 新增 `internal/apps/openflare/netquality/histogram.go`：
     - 实现 `LogHistogram64`（`HistVersion = 1`，`[0.5, 10000]ms` + 2 个溢出桶，零分配 `Add(ms)`、`MergeWeighted(other, weight)`、`Quantile(q)`、`SampleCount()`、`Encode()` / `Decode()`）。
   - 新增 `internal/apps/openflare/netquality/prefix_sketch.go`：
     - 实现 `PrefixSketch`（容量 64 的无符号 64 位脱敏前缀哈希集合，`HashIPPrefix(ip)` 取 IPv4 `/24` 或 IPv6 `/48` 加盐 FNV-1a 哈希，`Add`、`Union`、`DistinctCount()`、`Encode()` / `Decode()`、`saturated` 标志）。

3. **Goose 三库迁移与 Model / Repository / Logstore 对齐**
   - 新增迁移 `202611270003_refine_network_quality_phase1_5.sql`（`postgres` & `sqlite`）与 `202611270002_refine_network_quality_samples.sql`（`clickhouse`）：
     - `of_nodes`：新增 `geo_source`、`network_group`；
     - `of_network_quality_samples`（PG / SQLite / CH 三端列名一致）：新增 `client_region`、`ip_family`（默认 `'unknown'`）、`stats_version`（默认 `1`）、`rtt_hist`、`prefix_sketch`、`distinct_prefix_count`、`client_lat`、`client_lon`、`geo_accuracy_km`、`geo_source`，并新增复合索引 `(node_id, client_asn, client_country, client_region, ip_family, captured_at DESC)`；
     - `of_network_quality_matrix`（PG / SQLite）：新增 `client_region`、`ip_family`（默认 `'unknown'`）、`aggregation_level`（默认 `'country'`）、`stats_version`（默认 `1`）、`measurement_source`、`rtt_ewma_ms`、`distinct_prefix_count`、`geo_base_rtt_ms`、`geo_accuracy_km`、`geo_source`、`geo_confidence`、`last_observed_at` 及独立主动探测列 `probe_rtt_p50_ms`、`probe_loss_rate`、`probe_sample_count`、`probe_proto`、`probe_last_seen_at`；删除旧唯一索引 `uidx_of_nq_matrix_node_asn`，创建新唯一索引 `uidx_of_nq_matrix_node_asn_geo_v2 (node_id, client_asn, client_country, client_region, ip_family, aggregation_level)`（保留现有行待首轮 Rollup 事务替换，无冷启动空窗）；
     - `of_traceroute_records`：新增 `target_region`、`ip_family`。
   - 同步更新：
     - `internal/model/openflare_network_intel.go`、`internal/model/analytics/network_quality.go`
     - `internal/repository/openflare_network_intel.go`（支持六元组冲突 Upsert、单事务快照替换 `ReplaceOpenFlareNetworkQualityMatrixSnapshot`、新过滤/排序参数）
     - `internal/repository/logstore/`（`postgres_store.go`、`ch_node_observability.go`、`ch_node_observability_writer.go`、`clickhouse_store.go`）及 `internal/apps/openflare/tasks/log_db_switch_test.go`、`internal/testhelper/test_helper.go`、`internal/infra/persistence/migrator/migrator_test.go`。

4. **被动采集与聚合 Rollup 重构**
   - `internal/apps/openflare/agent/observability.go`：
     - 从 `RemoteAddr` 精确解析 `ipFamily`（`ipv4` / `ipv6`）、`country`、`region`、坐标与精度、`geoSource` 及脱敏前缀哈希，传入 `netquality.RecordPassiveSample`。
   - `internal/apps/openflare/netquality/passive_collector.go`：
     - 桶键升级为 `(nodeID, clientASN, clientCountry, clientRegion, ipFamily)`（桶中保留原始/规范化 `region` 状态）；
     - 写入 `LogHistogram64`、`PrefixSketch` 与 `SphericalCentroid`；
     - `maxPassiveBuckets = 8192`，达到 80% 水位时通过 `atomic.Bool` 防重入触发非阻塞提前 `FlushNow`；满载时优先淘汰 `sampleCount == 1` 的最旧冷桶，并统计与记录淘汰/丢弃指标。
   - `internal/apps/openflare/netquality/rollup_task.go`、`auto_probe_task.go`（采集入库部分）：
     - 主动探测写入样本也补充 `ip_family`、`target_region`、`stats_version = 2`、直方图与 `prefix_sketch`；
     - Rollup 严格按 `source` 分源合并 `LogHistogram64` 与 `PrefixSketch`；按 `captured_at` 升序计算 `rtt_ewma_ms`；`stats_version = 1` 样本按 `(node_id, client_asn, client_country, ip_family)` 兼容降权输出；
     - 同时生成 `aggregation_level = 'region'`（白名单国家）与 `aggregation_level = 'country'`（全部样本含区汇总）两级实测行；
     - 修复 `latestTracePaths` 匹配键为 `(node_id, target_asn, target_country, target_region, ip_family)`；
     - 计算 `geo_base_rtt_ms`、`geo_confidence` 与纯测量 `effective_rtt_ms`（按来源优先级 `tcp_passive > tcp_active > icmp_active/traceroute`），**禁止写入预测行到矩阵**，通过单事务原子更新并清退旧代际。

---

### Phase 1.5B：延迟预测引擎与离线回测验证（核心算法）

> **目标**：建立基于地理基线与网络残差的五级预测引擎，引入 `prediction_cache` 与六维回测体系。

1. **多维网络相似度与节点网络画像**
   - `pkg/netintel/upstream.go`：
     - 封装 `ASUpstreamProfile`（直连上游集合、CAIDA/RIPEstat 邻居、Tier-1 集合、运营商类型/Tag）；
     - 实现 `ComputeNetworkSimilarity(a, b, observedASPathSimilarity)`：任一或双侧缺失上游数据时显式返回 `Known: false, Score: 0`（严禁空集返回 `1.0`）；权重分层：直连上游重合 + 实测 `ASPath` 相似度（高） > 运营商类型/Tag（中高） > 远端 Tier-1 上游（低） > 仅 1~2 跳 AS 邻接（极低）。
   - `internal/apps/openflare/netquality/node_profile.go`：
     - 整合节点 `ASN`、`Country`、`Region`、`NetworkGroup`、`ObservedUpstreams`、`GeoLatitude`/`GeoLongitude` 构建轻量 `NodeNetworkProfile`。

2. **五级预测流水线与 `prediction_cache`**
   - 新增 `internal/apps/openflare/netquality/predictor.go`：
     - 统一输出 `PredictionResult`：拆分 `MeasurementSource`（`tcp_passive` / `tcp_active` / `icmp_active` / `traceroute` / `none`）与 `PredictionMethod`（`exact_region` / `country_rollup` / `inferred_upstream` / `similar_node` / `geo_prior`），含 `RTTMs`、`RTTP95Ms`、`CandidateDispersionMs`、`ErrorMs (*float64)`、`Confidence`、`SampleCount`、`DistinctPrefixes`、`LastObservedAt`、`GeoBaseRTTMs`；
     - Level 1（`exact_region`）与 Level 2（`country_rollup`）严格校验最低有效样本量与新鲜度门槛（陈旧单条被动样本不压过新鲜主动测量）；
     - Level 3（`inferred_upstream`）：同节点实测行提取纯残差 $RTT_{\text{extra}} = RTT_{\text{P50}} - RTT_{\text{geo\_base}}$（保留负残差并降低其地理权重），按 `w_geo × w_network × w_freshness × w_quality` 求加权中位数并与先验 $RTT_{\text{geo}}$ 做 Shrinkage 平滑融合，输出候选加权 IQR 作为 `CandidateDispersionMs`；
     - Level 4（`similar_node`）：当前节点实测稀少时按 `同 NetworkGroup + 同城` > `同 node_asn + 同城` 降权迁移；
     - Level 5（`geo_prior`）：$RTT_{\text{base\_geo}} \times 1.35 + 8\text{ms}$（坐标未知返回带高不确定度标识的默认估计，不输出 `0ms`）。
   - 使用 `pkg/cache/ram` 实现有界 TTL `prediction_cache`，绑定 Rollup 代际版本号并在 `RunRollup` 结束后自增版本号自动失效。

3. **六维离线回测与误差校准套件**
   - 新增 `internal/apps/openflare/netquality/predictor_eval_test.go`（与可复用评估 Runner）：
     - 覆盖 6 类留出场景：**随机缺口、全新 ASN、跨国 ASN 未见国家、新节点冷启动、时间前后切分、同 ASN 异质子网**；
     - 重点在多节点相近时间可比样本上统计 **Selection Regret（均值 & P95）**、**Top-1 / +5ms 命中率**、**MAE / P90**；
     - 构建基于回测残差分桶的初始误差校准映射，决定何时允许填充 `ErrorMs`。

4. **API 适配（`nettools`）**
   - `internal/apps/openflare/nettools/logics.go`、`routers.go`：
     - `TraceASPath` 支持 `target_country`、`target_region`、`ip_family`（未填且提供 `target_ip` 时自动提取），接入 `predictor.Predict` 输出预测方法、来源、置信度、离散度与误差；
     - `ListQualityMatrix` 支持 `client_region`、`ip_family`、`aggregation_level`、`measurement_source` 过滤与排序；
     - 新增 `/api/v1/d/network-tools/quality-matrix/predict` 按需预测对比接口。

---

### Phase 1.5C：受控主动探测、安全防护与可视化（补齐覆盖率）

> **目标**：废除盲 Ping 前缀 `.1`，构建节点隔离的受控靶点调度、SSRF 双向防线与控制台 UI。

1. **受控靶点表与按节点独立失败状态**
   - Goose 迁移（PG / SQLite，DDL 与 Seed 分文件）：
     - `202611270004_create_probe_targets_and_states.sql`：创建 `of_probe_targets`（唯一索引 `(ip, probe_proto, port)`，含 `ip_family`、`asn`、`country`、`region`、`target_type`、`anycast_status`、`enabled`）与 `of_probe_target_states`（唯一索引 `(node_id, target_id)`，含 `consecutive_failures`、`last_probed_at`、`last_success_at`、`next_probe_at`、`last_error`）；
     - `202611270005_seed_default_probe_targets.sql`：写入安全公共锚点种子数据（如 `1.1.1.1`、`8.8.8.8`、`9.9.9.9` 等带明确 `anycast_status = 'confirmed'` 标注）。

2. **Control Plane + Edge 双向 SSRF 校验与预算**
   - Go 控制面：靶点 CRUD API 挂载在 `openFlareAdminMiddlewares()`，复用 `nettools/safety.go` 校验公网 IP 与端口；
   - Rust Edge 端（`shared/probe/src/`）：在 Ping / TCPing / Traceroute 执行前增加**二次 IP/CIDR 安全校验**（禁止回环、RFC1918 私网、链路本地 `169.254.0.0/16` / `fe80::/10`、CGNAT `100.64.0.0/10`、保留基准网段 `198.18.0.0/15`、组播及未指定地址）；
   - 增加全局、单节点、单目标发包频率与并发预算控制。

3. **探索与不确定度驱动的自动探测调度（`auto_probe_task.go`）**
   - 彻底删除 `firstProbeablePrefixIP`；
   - 综合调度优先级：**流量热度 × 预测低置信度/高残差 × 数据陈旧度 × 非最优节点低频探索**（避免智能调度锁死导致备用节点失明的采样偏差）；
   - 按 `(node_id, target_id)` 维护指数退避，超时仅更新 `of_probe_target_states` 退避时间，绝不向 `of_network_quality_matrix` 写入 `0ms` 或不可达；
   - 仅 Traceroute `Reached == true` 计入终点 RTT；`anycast_status == 'confirmed'` 靶点结果与 `of_as_topology_edges` 及跨区域矩阵泛化严格隔离，`unknown` 仅作低置信度参考。

4. **前端控制台、i18n、文档与全套质量门禁**
   - `frontend/lib/services/openflare/`、`frontend/app/(main)/network-tools/quality/`、`frontend/app/(main)/network-tools/as-path/`、`frontend/app/(main)/nodes/`：
     - 质量矩阵 Tab 增加 `Country / Region / Level / IPFamily` 筛选与展示、拆分 `measurement_source` 与 `prediction_method`、`P50 / P95`、`Confidence` 与按需延迟预测模拟面板；
     - 新增「探测靶点」管理 Tab（列表、新增/启停、查看按节点维度退避与最近成功状态）；
     - 同步更新 `zh-CN` / `en` / `ru` fragments 并执行 `merge-i18n-fragments.mjs` 与 `check:i18n`；
   - 文档与收口：
     - `make swagger`、`make license`、`make format`；
     - 更新 `docs/guide/network-tools.md`，新增 Phase 2 SmartDNS ECS 接入契约设计文档（`docs/design/`），更新 `docs/changelog/index.md` 与 `.agents/notes/`；
     - 全量验证：Go 相关包测试、Rust `shared/probe` & `edge` 测试、`golangci-lint`、前端 `check:i18n` / `tsc` / `eslint`。
