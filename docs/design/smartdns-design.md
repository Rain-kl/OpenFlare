# SmartDNS 系统架构设计

OpenFlare SmartDNS 结合控制面多源网络质量矩阵、节点负载容量、视觉化 JDM 调度规则引擎以及支持 EDNS0 Client Subnet（ECS）的权威 DNS 数据面，为多边缘节点与外部指向域名提供低延迟、支持多区域感知与过载保护的智能解析服务。

---

## 1. 总体拓扑与防腐边界

* **控制面与数据面解耦**：SmartDNS 控制面（`internal/apps/openflare/smartdns`）与独立 Rust 权威服务端（`dns/`）均不直接读取 `netquality` 或 `pkg/netintel` 的底层表结构，而是统一经由防腐契约 `internal/apps/openflare/qualityapi.Provider` 获取预测结果与 `CompactQualitySnapshot`；Rust 侧对应定义为 `dns/src/quality.rs::QualityOracle`。包依赖隔离由 `internal/apps/openflare/smartdns/imports_test.go` 在单测中强制检查。
* **快照编译与离线容灾**：
  * `SnapshotManager` 将在线且未隔离的节点（含 tags、容量上限、当前负载）、启用中的调度策略（含编译态 JDM JSON）、`target_mode = smartdns` 的域名路由映射及紧凑质量快照 `CompactQualitySnapshot` 聚合为带单调递增版本号与强 `ETag` 的 `DNSSnapshotEnvelope`；
  * 任何节点、策略或指向变更均经领域事件 `listener.EmitSmartDNSSnapshotDirty` 触发 250 ms 防抖重建，并通过 `/api/v1/dns-agent/ws` 推送 `dns:snapshot_changed` 唤醒各实例增量/ETag 拉取（同时保持默认 10 秒快照检查与心跳上行）；
  * 独立 `openflare-dns` 实例将最近一次有效快照持久化至 `--cache-dir/last-snapshot.bin`，控制面短暂不可达时仍可持续响应权威查询。

---

## 2. 权威解析数据面 (`openflare-dns` 与内嵌引擎)

系统支持 **独立 Rust 高性能实例 (`openflare-dns`)** 与 **控制面内嵌 Go 引擎** 两种运行形态，共享一致的查询语义与评分规则：

* **网络监听与协议合规（`dns/src/server.rs`）**：基于 `hickory-server`，在 Unix 平台按工作线程数创建多路 `SO_REUSEPORT` UDP Socket 与 TCP 监听器，权威应答 `A`、`AAAA`、`NS`、`SOA`；对非受控权威域返回 `REFUSED`，无对应记录类型返回携带 `SOA` 的 `NODATA`。
* **EDNS0 ECS 感知、分片 RRL 与 L1 缓存（`dns/src/cache.rs`）**：优先提取报文中的有效非私网 ECS 前缀用于归属查库与决策，并在应答中规范回传 `/24`（IPv4）或 `/48`（IPv6）作用域前缀；基于标准化 `ClientPrefixKey` 运行 64 分片令牌桶限流（RRL）及 `moka` L1 应答缓存（按 `version + qname + qtype + prefix + from_ecs` 隔离）。
* **JDM 非阻塞隔离（`dns/src/engine.rs`）**：`zen-engine` 编译 JDM 规则图；对含 `!Send` `Future` 的规则求值运行在隔离阻塞线程池的 `current_thread` 运行时中，且仅在 L1 缓存未命中时触发。
* **远程自更新与双模式运行**：`openflare-dns` 支持静态配置、心跳携带 `UpdateReceipt` 以及在托管环境下接收远程升级指令（`update.rs`）平滑切换版本。

---

## 3. 双端同源的 JDM 编排与 4 阶段选点算法

控制面 Go（`smartdns/scoring.go`、`embedded_go_dns.go`）与数据面 Rust（`dns/src/engine.rs`）共享同一套 JDM 输出语义与 4 阶段选点算法：

1. **条件过滤与策略执行**：依据匹配规则的 `filters`（`node_ids`、`countries`、`tags`、`network_groups`）筛选在线且未隔离的候选节点，并支持 `smart` 与 `load_balance` 两种主模式。
2. **多维度评分模型**：
   * **`LoadIndex`**：由节点 CPU 利用率（`45%`）、内存利用率（`25%`）与带宽/连接容量占比（`30%`）加权合成；综合负载 `> 0.92` 时标记为过载并移出主候选池。
   * **质量延迟分 `S_lat`**：融合 `effective_rtt_ms`、动态 `p95_weight` 尾延迟罚分与 `loss_rate` 惩罚；Rust 数据面按「实测区域（`region`）→ 实测国家（`country`）→ 同国跨 ASN 回落 → 节点默认先验 / 地理距离基线」快速查表（详见[网络情报与链路质量感知架构](./network-quality-design.md)）。
   * **地理邻近分 `S_geo`**：基于 Haversine 球面距离衰减计算（尺度 `2000 km`）。
   * **标签权重与确定性负载均衡**：开启 `use_tag_weights` 时按节点绑定标签乘数缩放（区间 `[0.25, 4.0]`）；`load_balance` 模式基于客户端前缀、域名与节点 ID 执行确定性加权伪随机排序（`Efraimidis-Spirakis` 风格）。
   * **受限探索预算**：在满足多候选且探测收益条件时，以 `0.5%–3%` 确定性时间桶预算探索次优/待验证节点（保守封顶 TTL `25s`），避免节点因样本冷门永久饥饿。
3. **5% 防抖粘滞保护（Hysteresis）**：对相同决策键在 `10 分钟` 窗口内记录现役节点；若候选最优节点分值相比当前粘滞节点未超过 `1.05×`（`5%` 门限）且现役节点未过载，则保留原节点以抑制 DNS 抖动。
4. **强制回退保障**：即使所有筛选节点均过载或条件过滤为空集，仍按「策略静态备选节点 → 过载但在线节点 → 未筛选在线节点 → 未隔离节点」依次兜底，保证至少返回可用记录。

---

## 4. 可观测性汇总与存储路由

独立 `openflare-dns` 实例通过周期心跳上报 QPS、P50/P95/P99 分位延迟（`hdrhistogram` 16 分片无锁直方图统计）、查询状态计数、缓存命中率、快照版本与 CPU/内存占比；控制面写入 `logstore.Observability.InsertDNSMetricSnapshot`，遵循项目统一双库路由进入 ClickHouse 或 PostgreSQL/SQLite 的 `of_dns_metric_snapshots` 表。
