# SmartDNS Phase 1.5：精确网络质量测量与延迟预测实现记录

## 关键变更与架构摘要

### 1. Phase 1.5A（数据正确性与采集底座）
- **多维键与分布合并**：解决同 ASN 跨国及 IPv4/IPv6 路径折叠问题，样本与矩阵引入 `client_region`、`ip_family` 及两级 `aggregation_level`（`region` 对高地域跨度白名单国家保留省/州粒度，`country` 统一兜底汇总）。用 `LogHistogram64`（64 对数分桶，相对误差 `< 8%`）与 Bottom-64 `PrefixSketch` 替换有偏分位数平均与固定 `1.0:0.7` 跨协议混权；用时间戳排序 EWMA 与球面三角向量质心替代算术平均。
- **Geo 精度与线路隔离**：`geoip` 节点坐标回填优先使用 City MMDB，新增 `geo_source` 与节点线路标识 `of_nodes.network_group`（用于区分同一城市内的 `cn2-gia`/`cmi`/`bgp` 产品线路）。
- **并发与隔离**：被动收集器引入 8192 桶冷启动 LRU/驱逐与 80% 高水位单次提前刷盘，`ResetCollectorForTest()` 通过等待 `flushWG` 杜绝异步刷盘竞态污染后续 SQLite 测试实例；Rollup 仅落盘 `measured` 行并递增 `RollupGeneration` 触发预测缓存失效。

### 2. Phase 1.5B（5 级预测器与离线回测）
- **多维网络相似度**：在 `pkg/netintel/upstream.go` 提取 AS 上游画像（直接上游、Tier-1、类型、国家、标签），当上游与标签数据均缺失时明确返回 `Known: false`，仅共享 Tier-1 时给予低权，避免无依据相似迁移。
- **5 级分级预测 + 缓存 + 误差校准**：实现 `exact_region` → `country_rollup` → `inferred_upstream`（严格基于同 `network_group` 过滤且阻断 `AnycastConfirmed` 跨越泛化） → `similar_node`（同城 `<= 50km` 且同 `network_group`） → `geo_prior`；配 `pkg/cache/ram` 有界 TTL 缓存与留一法 P80 `ErrorCalibration`。
- **6 场景离线回测验证**：`predictor_eval_test.go` 验证随机缺口、未见 ASN、跨国未见国家、冷启动同城同组继承、时序划分及同 ASN 异质区域/IPv6 六类场景均显著优于旧 `as_hops*14+12` 估算。

### 3. Phase 1.5C（受控靶点、双向 SSRF 与 UI）
- **可控靶点与节点级退避**：新增 `of_probe_targets`（公网靶点与 `anycast_status` 元数据）与 `of_probe_target_states`（按 `(node_id, target_id)` 独立跟踪连续失败与指数退避），彻底移除 `firstProbeablePrefixIP`。调度综合流量热度、预测低置信度/残差离散度、陈旧度与非最优节点探索奖励，失败/超时仅更新退避状态而绝不写入 `0ms` 伪样本，`confirmed` Anycast 锚点严格隔离出 AS 拓扑与区域外推。
- **双向 SSRF 防护与预算**：Go 管理接口与 Rust `shared/probe/src/safety.rs` 同步封锁回环、RFC1918、链路本地、CGNAT（`100.64.0.0/10`）、基准测试网段（`198.18.0.0/15`）、组播、未指定地址及内部域名，禁止 HTTP 自动重定向并限制滑动窗口发包预算。
- **Rust 测试陷阱与隔离方式**：将公开 `execute_ping` / `execute_ping_batch` / `execute_net_probe` / `execute_dns_query` / `run_probe_worker_with_io` 默认锁定为严格 SSRF 策略；仅在内部测试中通过 `*_with_policy(..., false)` 验证本地环回监听器统计与 TCP 回落逻辑，避免发布构建存在任何运行期绕过开关，同时确保 `udp_truncation_falls_back_to_tcp` 不会因环回被拒导致后台接收任务永久挂起。

## 后续联调与演进参考
- Phase 2 SmartDNS 规则与 ECS 数据面接入契约见 [`docs/design/smartdns-phase2-ecs-contract.md`](../../docs/design/smartdns-phase2-ecs-contract.md)。
