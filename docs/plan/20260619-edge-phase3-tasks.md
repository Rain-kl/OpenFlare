# 边缘运行时 Phase 3 — 任务拆解

> **状态**：Batch 1 + Batch 2 已完成（2026-06-19）
> **前置**：[边缘运行时重构设计](../design/edge-runtime-refactor.md) Phase 0–2 已完成

---

## 任务依赖图

```text
Batch 1（并行，互不影响）
├── T1  edge/config/duration.go
├── T2  edge/observability/linux.go
└── T3  edge/heartbeat/loop.go（仅 relay/flared）

Batch 2（串行，依赖 Batch 1 或需独立评审）
├── T4  Agent heartbeat 架构对齐（runner ↔ heartbeat service）
└── T5  agent/protocol → pkg/protocol（影响 Server 侧）
```

---

## Batch 1 — 并行任务

| ID | 任务 | 修改范围 | 风险 | 委派 |
| --- | --- | --- | --- | --- |
| **T1** | 抽取 `MillisecondDuration` | `edge/config/` + `agent/relay/flared/config` | 低 | ✅ 子代理 A |
| **T2** | 抽取 Linux 指标采集 | `edge/observability/` + `agent/relay/observability/collector.go` | 中 | ✅ 子代理 B |
| **T3** | 统一心跳 ticker 循环 | `edge/heartbeat/loop.go` + `relay/flared/heartbeat` | 低 | ✅ 子代理 C |

### 隔离规则

- **T1** 禁止修改 `heartbeat/`、`observability/`、`agent/runner.go`
- **T2** 禁止修改 `config/`、`heartbeat/`
- **T3** 禁止修改 `agent/` 任何文件（Agent 心跳留在 runner，Batch 2 处理）

---

## Batch 2 — 并行任务（已完成）

| ID | 任务 | 修改范围 | 状态 |
| --- | --- | --- | --- |
| **T4** | Agent heartbeat 架构对齐 | `heartbeat/cycle.go` + 精简 `agent/runner.go` | ✅ 子代理 D |
| **T5** | `agent/protocol` → `pkg/protocol` | `pkg/protocol/agent.go` + `protocol/alias.go` | ✅ 子代理 E |

---

## 验收标准（Batch 1）

```bash
go build ./cmd/agent ./cmd/relay ./cmd/flared
go test ./internal/apps/edge/... ./internal/apps/agent/... ./internal/apps/relay/... ./internal/apps/flared/... -count=1
```