# SmartDNS Phase 2 系统架构设计

本文档说明 OpenFlare SmartDNS Phase 2 的控制面与数据面分层设计、反腐败边界（Anti-Corruption Layer）、JDM 规则编排一致性、4 阶段调度评分模型以及可观测性汇总链路。

---

## 1. 总体拓扑与反腐败边界

- **控制面与数据面解耦**：SmartDNS（`internal/apps/openflare/smartdns` 及独立 Rust `dns/`）不直接读取 `netquality` 或 `pkg/netintel` 底层存储表，而是通过防腐契约接口 `internal/apps/openflare/qualityapi.Provider` 获取归一化的质量预测与 `CompactQualitySnapshot`；Rust 侧对应定义为 `dns/src/quality.rs::QualityOracle`。边界通过 `internal/apps/openflare/smartdns/imports_test.go` 在单测中强制校验。
- **状态分发与离线容灾**：
  - 控制面聚合在线节点、节点标签、容量、调度策略（含 JDM JSON）、Cloudflare SmartDNS 路由映射以及紧凑质量快照 `CompactQualitySnapshot`，构建带版本号与强 `ETag` 的全量快照；
  - 变更时经内部领域事件 `listener.EmitSmartDNSSnapshotDirty` 触发防抖重建，并通过 `/api/v1/dns-agent/ws` WebSocket 下发 `dns:snapshot_changed` 唤醒，辅以增量/ETag HTTP 拉取与 10 秒心跳上行；
  - 独立 Rust 实例持久化离线快照至 `--cache-dir/last-snapshot.bin`，控制面短暂不可达时仍可持续提供权威解析服务。

---

## 2. 权威解析数据面 (`openflare-dns`)

- **网络监听与协议合规**：基于 `hickory-server 0.26.4`，在 Unix 平台按 CPU 核数开启多路 `SO_REUSEPORT` UDP Socket 与 TCP 监听器，处理 `A`、`AAAA`、`NS`、`SOA`，对非本权威范围域名返回 `REFUSED`，无记录类型返回合规 `NODATA`（带 `SOA`）。
- **EDNS0 ECS 处理**：优先提取请求报文中的非 `0` 前缀 ECS 地址用于地区/ASN 查询与决策，应答报文规范回传 `/24`（IPv4）或 `/48`（IPv6）作用域前缀；并基于标准化前缀字节键（`ClientPrefixKey`）做零堆分配分片 RRL 令牌桶限流与 `moka` L1 应答缓存。
- **并发与异步隔离**：由于 `zen-engine 2.1.8` 的 `Decision::evaluate` 内部包含 `!Send` 状态的 `Future`，Rust 引擎将其隔离运行于 `tokio::task::spawn_blocking` 的线程局部单线程 `current_thread` 运行时中，同时仅在 L1 缓存未命中时触发。

---

## 3. 双端同源的 JDM 编排与 4 阶段选点算法

控制面 Go 与数据面 Rust 共享同一套 JDM 输出参数语义与 4 阶段评分逻辑：

1. **条件过滤与策略执行**：依据政策筛选条件过滤在线且未隔离的候选节点（`node_ids`、`countries`、`tags`、`network_groups`）。
2. **多维度评分模型**：
   - **LoadIndex**：由节点 CPU 利用率（45%）、内存利用率（25%）与带宽/连接数上限占比（30%）加权合成；综合负载高于 `0.92` 时标记为过载并移出主候选池。
   - **质量延迟分 `S_lat`**：综合 `EffectiveRTTMs` 与 `LossRate` 惩罚；支持 4 级查找回落（实测区域 → 实测国家 → 同国跨 ASN → 地理基线先验）。
   - **地理邻近分 `S_geo`**：Haversine 球面距离衰减函数。
   - **标签权重乘数与容量权重**：当启用标签权重时按绑定标签的有效权重乘积缩放（区间 `[0.25, 4.0]`）；负载均衡模式下采用基于客户端前缀、域名与节点 ID 的确定性伪随机加权排序（`Efraimidis-Spirakis` 风格）。
3. **5% 防抖粘滞保护（Hysteresis）**：对于相同 `(policy_id, client_prefix, qtype)`，当最佳新节点得分相对当前粘滞节点提升幅度不超过 `5%`（`<= 1.05x`）且原节点未过载时，保留原节点应答以消除路由抖动。
4. **强制回退保障**：即使全部节点处于过载状态（`> 0.92`）或严格筛选结果为空，算法依次沿「静态备选节点 → 过载但在线节点 → 未筛选在线节点 → 未隔离节点」保证至少返回有效候选 IP。

---

## 4. 指标汇总与多库支持

- 实例心跳与内嵌引擎统计汇总至内存时间窗口后写入 `logstore.InsertDNSMetricSnapshots`，遵循项目统一双库规则：配置 ClickHouse 时异步批量写入 ClickHouse `of_dns_metric_snapshots`，否则写入业务库 PostgreSQL / SQLite，并支持在线切换日志库。
- 详见前置质量感知契约：[SmartDNS Phase 2 ECS 质量预测器契约设计](./smartdns-phase2-ecs-contract.md)。
