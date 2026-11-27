# SmartDNS 自适应质量状态机、动态 P95 与 ECS 探索设计

## 背景与目标

在纯被动 `TCP_INFO` 探测为主的调度体系中，原有单窗口 `24h / 6h 半衰期` Rollup 与赢家通吃选路存在三类问题：
1. **晚高峰切走滞后**：白天低延迟样本在 6h 半衰期下权重过高，晚高峰突发拥塞需要数小时才能触发切线；且 `maxRollupSamples = 20000` 全局上限会在多节点场景下截断长尾分组数据。
2. **陈旧节点死锁或误切**：流量切走后备用节点不再产生被动样本；若按时间流逝直接调低延迟会造成仍拥塞线路瞬间抢回主流量，而一直保留旧惩罚则导致白天恢复后无法感知。
3. **DNS 灰度缓存放大**：权威 DNS 面临无 ECS 公共递归器的大范围缓存放大风险，固定均匀 3% 灰度也会把预算浪费在长期明显更慢或无恢复潜力的节点上。

## 核心改动

1. **键集分页 + 三层时间窗 + 四态状态机**（[adaptive_state.go](../../internal/apps/openflare/netquality/adaptive_state.go)、[rollup_task.go](../../internal/apps/openflare/netquality/rollup_task.go)）：
   - `loadPaginatedQualitySamples` 通过 `(captured_at, id)` 键集分页拉取最近 24h 样本（每页 5,000 条，上限 100,000 条），兼容 PostgreSQL、SQLite 与 ClickHouse `logstore`。
   - 同时统计 10 分钟即时窗口、45 分钟短期窗口（20 分钟半衰期）与 24 小时基线窗口（6 小时半衰期），以及基于 `tcp_min_rtt_ms` / `geo_base_rtt_ms` 的无拥塞预测 `predicted_rtt_ms`。
   - 引入 `quality_state`（`healthy` / `degraded` / `recovering` / `stale`）：
     - 即时/短期窗满足劣化阈值时立即转 `degraded`，`effective_rtt_ms` 立刻切换为当前实测；
     - 超过 60 分钟无新观测转 `stale`，保留已有受损延迟并仅按 2 小时半衰期衰减 `confidence`；
     - 受损节点重新获得正常观测时，仅当 `last_observed_at` 相对上一轮推进且连续 2 个新鲜窗口恢复正常，才由 `recovering` 转 `healthy` 并切换 `effective_rtt_ms`。
2. **动态样本量/前缀数 P95 加权**（Go `netquality`/`smartdns` 与 Rust `dns/`）：
   - `computeDynamicP95Weight` 根据 `sample_count >= 10` 与 `distinct_prefix_count` 在 `[0, 0.35]` 平滑缩放尾部权重，避免小样本偶发高延迟误判整条线路，并在两端评分中统一消费 `p95_weight`。
3. **价值导向 ECS 探索**（`assignExplorePriorities` + `smartdns/scoring.go` + `dns/src/engine.rs`）：
   - 控制面在每个客户端切片内，仅为在线且预测有竞争力（`predicted_rtt_ms <= best * 1.5 + 30ms`）的 `recovering` / `stale` / 冷却到期 `degraded` 节点赋予 `explore_priority ∈ (0, 1]`；收到充足近期样本且未恢复的节点在 15 分钟冷却期内自动刹车为 `0`。
   - Go 与 Rust 数据面仅在 `from_ecs == true` 的查询上启用基于 `(client_prefix, domain, 900s bucket)` 的 SHA-256 确定性探索预算（`0.5%–3%`），探索结果 TTL 收敛至 `<= 25s`、跳过 `HysteresisTracker` 写入，并以 `from_ecs` 隔离 L1 应答缓存。

## 关键约束与防回退要点

- 保持 `smartdns -> qualityapi` 防腐边界，禁止在 `smartdns` 直接引用 `netquality` 或 `OpenFlareNetworkQualityMatrix`。
- 不在 `ExportSnapshot` 中按实时 `time.Now()` 二次衰减 `confidence`，避免每 10 秒触发无关 ETag 变化。
- 探索选中节点绝不能写入 `HysteresisTracker`，防止小比例探测流量把备用节点粘滞为主节点。
