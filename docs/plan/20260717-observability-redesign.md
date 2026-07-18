# 边缘可观测与业务流量统计重构 — 实现计划

说明：本计划对应设计文档 [observability-design.md](../design/observability-design.md)。重大架构重构，按阶段交付，避免一次大爆炸。

---

## 0. 落地进度（2026-07-18）

* [x] M1 看板业务趋势改读 access log；网络图文案改为已提供/接收 + 宿主机网卡
* [x] M2 协议 v2 字段（host_metrics/edge_health/request_length）；CH 列 `request_length`/`request_time_ms`
* [x] M3 Agent：观测口仅健康连接；payload 不再发 TrafficReport；access_logs 带 request_length
* [x] M4 Server：停写 TrafficReport；openresty 仅存 connections；明细入库带 request_length
* [x] 分布图 status/top domains + 节点行请求/UV 改 access log；24h UV 用 uniqExact；API bytes_provided/received
* [x] M5：`of_node_edge_health`、`of_access_log_hourly`(+MV)；删除 request_reports/traffic_hourly/openresty_hourly/obs_openresty；写入/查询改道
* [x] 收尾：清 openresty hourly / request_report 死路径；edge_health 写全 status；cleanup 命名 `node_edge_health`；hourly 回填 SQL + UV 策略文档
* [x] 协议/API 去兼容层（Agent 销毁重建）：删除 TrafficReport / openresty_observation / snapshot 别名 / request_reports API 字段 / openresty_rx|tx
* [x] 前端 UV 文案：24h/查询窗口独立访客；趋势图不绘分时 UV
* [ ] 真实环境 ClickHouse 迁移 + `202607180003` 回填（本机 Docker 未起时需运维执行）

## 1. 目标与背景 (Goal & Context)

* **需求背景**：看板「OpenResty 入/出站」与 Zone「已提供数据」不一致；Agent 预聚合与访问日志双轨；`openresty_tx` 与 `bytes_sent` 业务语义重复。
* **开发范围 (Scope)**：
  * **必做**：业务趋势统一为访问日志聚合；UI 字段与文案收敛；协议补齐 `request_length`；停用预聚合作为权威源；Agent 瘦身。
  * **后续**：废弃 CH 表清理、hourly rollup 性能优化、Relay 指标对齐。
* **Out of Scope**：通用日志平台、替换 ClickHouse、APM。

---

## 2. 设计与决策 (Design & Decisions)

* **核心对象**：以 `of_node_access_logs` 为 L1 权威；主机 snapshot 为 L3；OpenResty 仅健康/连接为 L2。
* **传输模型（示例与频率）**：见 [observability-transport-model.md](../design/observability-transport-model.md)。
* **协议与表结构**：见 [observability-data-model.md](../design/observability-data-model.md)（NodePayload v2、落库流水线、DDL、废弃表）。
* **API**：看板与 Zone 共用聚合语义；`bytes_provided` / `bytes_received`（兼容 `bytes_sent` 别名）。
* **数据流**：见 [observability-design.md](../design/observability-design.md) §5。
* **权衡**：性能用 Server 侧 rollup，不恢复 Agent 预聚合。

---

## 3. 阶段与修改清单 (Proposed Changes)

### 阶段 M1 — 读路径切换（优先对账）

* #### [MODIFY] `internal/apps/openflare/dashboard/*`、`observability/analytics.go`
  * 业务 24h 趋势改为 access log 聚合（全局）。
  * 网络趋势中业务曲线与主机网卡分离。
* #### [MODIFY] 前端 dashboard 组件与文案
  * 「OpenResty 出站/入站」→「已提供数据/接收数据」或拆卡片。
* #### [MODIFY] Zone stats 字段对齐（如需别名）
* **验收**：单 Zone 流量时看板已提供 ≈ Zone 已提供。

### 阶段 M2 — 协议与入库补齐

* #### [MODIFY] `pkg/protocol/agent.go` — `NodeAccessLog.request_length`
* #### [MODIFY] Agent 解析与 CH 写入列
* #### [MODIFY] goose ClickHouse migration（如缺列）

### 阶段 M3 — 停写预聚合权威路径

* #### [MODIFY] Server persist：TrafficReport / openresty rx/tx 不再驱动看板
* 可选：直接停写以减 CH 压力

### 阶段 M4 — Agent 瘦身

* #### [MODIFY] 移除 TrafficReport 构建主路径、Lua 业务 dict 计数、state 内业务累计
* #### [MODIFY] 心跳仅明细 + snapshot + 连接/健康

### 阶段 M5 — 清理

* 删除废弃 API 字段、前端类型、CH 表/MV、相关测试夹具
* 更新 agent-design / changelog（代码变更时）

---

## 4. 验证计划 (Verification Plan)

### 自动化

* `go test`：zone stats、dashboard 聚合、agent access log 解析
* 前端：zone / dashboard 文案与字段测试

### 手动

* 制造已知大小响应，对比 Zone 与看板 24h 已提供数据
* 确认宿主机网卡曲线与业务已提供数据分区展示、数值可不一致且文案不诱导对账

### 质量门禁

* `make swagger`（若 API 变更）
* `make code-check`
* `make prettier`

---

## 5. 依赖与风险

* 明细量大时 M1 需同步评估 hourly rollup（仍 Server 侧）。
* 旧 Agent 无 `request_length` 时接收数据为空，需 UI 降级。
