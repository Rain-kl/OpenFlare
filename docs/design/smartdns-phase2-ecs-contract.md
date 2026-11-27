# SmartDNS Phase 2：DNS / ECS 请求接入网络质量预测器契约设计

本文档定义 OpenFlare 智能 DNS 引擎（Phase 2，权威应答与 `gorules/zen-go` 规则编排）与 **Phase 1.5 网络质量感知层**（`internal/apps/openflare/netquality` + `pkg/netintel`）之间的运行时数据契约、降级规则与性能边界。

---

## 1. 上下文解析：从 DNS 请求 / ECS 提取查询画像

当权威 DNS 实例收到一条域名解析请求（如 `A` / `AAAA` 查询）时，按以下优先级确定请求归属 IP 与地址族：

1. **客户端归属 IP 提取**：
   - 若报文携带有效且非私网的 **EDNS0 Client Subnet (ECS)** 地址，则优先取 ECS 前缀地址作为决策客户端 IP；
   - 否则回落至 UDP/TCP 请求源 IP（`RemoteAddr`）。
2. **IP 地址族归一化（`IPFamily`）**：
   - 依据权威解析请求的查询 QTYPE 与客户端来源协同判定或传递 `pkgnetintel.DetectIPFamily(clientIP)`（`'ipv4'` / `'ipv6'` / `'unknown'`）。
   - 注意：IPv4 与 IPv6 路由路径在矩阵与预测器中严格隔离，不允许再用固定比例对不同地址族混算。
3. **无锁网络情报查询（`netintel.LookupIP`）**：
   - 单次无锁读取返回 `pkgnetintel.IPLookupResult`：
     - `ASN`、`ASNName`、`ASNTags`
     - `CountryISO`、`RegionCode`（按 `IsRegionDrilldownCountry` 下钻规则归一化，如 `CN-GD`、`US-CA`）
     - `Latitude`、`Longitude`、`AccuracyRadiusKm`、`GeoSource`（`city` > `country` > `asn_country`）
     - `AnycastStatus`（`confirmed` / `unicast` / `unknown`）

---

## 2. 预测器调用契约

SmartDNS 解析工作器或规则上下文注入器持有当前活动 `Predictor` 快照引用，按如下结构对筛选后的候选在线边缘节点逐一调用 `Predictor.Predict` 与 `ComputeQualityScore`：

### 2.1 输入结构（`netquality.PredictQuery`）

```go
query := netquality.PredictQuery{
    NodeID:        candidateNode.NodeID,
    ClientASN:     lookup.ASN,
    ClientCountry: lookup.CountryISO,
    ClientRegion:  pkgnetintel.NormalizeMatrixRegion(lookup.CountryISO, lookup.RegionCode),
    IPFamily:      pkgnetintel.DetectIPFamily(clientIP),
    ClientLat:     lookup.Latitude,
    ClientLon:     lookup.Longitude,
    GeoAccuracyKm: lookup.AccuracyRadiusKm,
    GeoSource:     lookup.GeoSource,
    AnycastStatus: lookup.AnycastStatus,
}
res := predictor.Predict(query)
score := netquality.ComputeQualityScore(res)
```

### 2.2 输出语义（`netquality.PredictionResult`）

| 字段 | 类型 | 消费约定（DNS / 策略引擎） |
| :--- | :--- | :--- |
| `RTTMs` | `float64` | 节点到访客网络预计中位有效延迟（ms），始终保持未扣减离散度惩罚的物理可读估计值，可用于规则表达式阈值判断（如 `rtt_ms < 80`）。 |
| `RTTP95Ms` | `float64` | P95 尾延迟估计（ms），适合对长尾敏感的业务策略评分。 |
| `CandidateDispersionMs` | `float64` | 第 3 级相似上游或第 4 级同城节点推断中候选残差的加权四分位距（IQR），用于反映预测分歧风险。 |
| `ErrorMs` | `*float64` | 来自离线留一法校准表的经验 P80 绝对误差；未经充分样本校准时明确为 `nil`，规则引擎须做空值判断，不得假设默认误差。 |
| `Confidence` | `float64` | 归一化置信度 `[0.05, 0.99]`。随样本时效、直方图版本（V2）、前缀多样性与地理精度衰减。 |
| `MeasurementSource` | `string` | 实测物理信号来源（`tcp_passive`、`tcp_active`、`icmp_active`、`traceroute`、`none`）。L3～L5 推断一律为 `none`。 |
| `PredictionMethod` | `string` | 预测命中的级别（`exact_region` → `country_rollup` → `inferred_upstream` → `similar_node` → `geo_prior`）。 |
| `GeoBaseRTTMs` | `float64` | 节点到目标网络的物理光纤基线延迟（`2d / 200` ms）。当节点与目标同城或同中心点时合法值为 `0`，需配合 `GeoBaseKnown` 判定坐标是否有效。 |
| `GeoBaseKnown` | `bool` | 地理基线是否基于有效节点与客户端/质心坐标计算。`true` 且 `GeoBaseRTTMs == 0` 表示同城/同位置 0 km 节点；`false` 表示坐标缺失（L5 回退 `120 ms` 未知先验）。 |
| `AnycastIsolated` | `bool` | 当目标/锚点被标注为 `confirmed` Anycast 时为 `true`，明确阻断跨区域或基于局部观测的错误迁移。 |

> **关键契约**：`score := netquality.ComputeQualityScore(res)` 已在内部评分时将 `0.25 * CandidateDispersionMs` 计入等效开销（但不篡改 `res.RTTMs`），规则引擎若直接使用 `RTTMs` 与 `CandidateDispersionMs` 进行自定义加权评分，不应重复叠加相同离散度惩罚。

---

## 3. 规则编排集成与回退边界（`gorules/zen-go`）

1. **两阶段流水线（先筛选，后评分）**：
   - **筛选阶段**：根据节点标签（优化线路、国际线路、大流量线路及对应 `network_group`）、所属国家、健康/隔离状态（`!IsQuarantined()` 且在 `online` 心跳窗口内）收敛候选集；
   - **评分阶段**：将每个候选节点的 `PredictionResult` 指标（`effective_rtt_ms`、`rtt_p95_ms`、`quality_score`、`confidence`、`geo_base_rtt_ms`、`prediction_method`）、主机负载与标签权重注入 Zen Decision Engine 上下文，按总分排序选取并返回 A / AAAA 记录；
   - **强制回退策略**：当筛选后候选为空或所有节点均处于退避/故障状态时，按域名策略绑定的回退模式执行（例如全局健康负载均衡、指定静态备选节点或特定地域回退）。
2. **ECS 应答作用域控制（EDNS0 Response Scope）**：
   - 当预测命中 `exact_region`（L1）且高置信度时，权威应答 `Scope Prefix-Length` 可保守收窄为 `/24`（IPv4）或 `/48`（IPv6），防止递归解析器过度放大跨省缓存；
   - 当回落至 `country_rollup`（L2）或更高泛化级别（L3～L5）且同国多地区差异较小时，可放宽范围标记或跟随策略默认 TTL（如 60s～300s）。

---

## 4. 一致性、缓存与性能 SLO

- **锁无关热切换与缓存失效**：
  - `of_netquality_rollup` 每次原子提交新矩阵快照并递增 `netquality.CurrentRollupGeneration()`，同步调用 `prediction_cache.InvalidateAll()`。
  - 预测缓存键为 `(generation, node_id, client_asn, client_country, client_region, ip_family, has_coords, lat_bucket, lon_bucket, geo_source)`（其中 `lat_bucket` / `lon_bucket` 按 `0.2°` 量化，避免同国家同省区不同城市共享缓存），TTL 5 分钟、有界容量 4096 条；对于 `AnycastStatus == 'confirmed'` 查询自动旁路 L3 推断缓存污染。
- **延迟与内存预算**：
  - 权威 DNS 数据面读取预测结果原则上基于内存 `Predictor` 快照，禁止在每条 DNS 查询协程中同步执行 SQL 查询；
  - 由后台定时器或订阅信号按 `CurrentRollupGeneration()` 变化异步刷新进程内 `Predictor` 实例（实测目标：缓存命中 `< 50 µs`，未命中全 5 级推断 `< 5 ms`）。
