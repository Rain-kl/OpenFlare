# 访问日志 cache_status 明细可见 — 实现计划

说明：对应设计 [observability-data-model.md §3.5.1](../design/observability-data-model.md)。第一期只做明细可见，不上报 upstream 地址。

---

## 1. 目标与背景

* **需求背景**：访问日志无法判断请求是否命中边缘缓存、是否回源。
* **开发范围 (Scope)**：
  * **必做**：OpenResty 日志输出 `$upstream_cache_status`；Agent 上报；CH 入库；列表/详情展示三态标签。
  * **Out of Scope**：命中率看板、hourly 维度、`upstream_addr`。

---

## 2. 设计与决策

* **唯一字段**：`cache_status` string（原始值）。
* **UI 三态（不落库）**：
  * 命中：`HIT` / `STALE` / `REVALIDATED` / `UPDATING`
  * 回源：`MISS` / `EXPIRED`
  * 未缓存：`BYPASS` / `-` / 空
* **数据流**：log_format → Agent parse → protocol → Server model → CH → API → 前端明细。

---

## 3. 修改清单

### 边缘 / 协议

* `pkg/render/openresty/types.go`、`internal/model/openflare_option.go`：`log_format` 增加 `cache_status`
* `internal/apps/agent/observability/traffic.go`：解析与映射
* `pkg/protocol/agent.go`：`NodeAccessLog.CacheStatus`

### Server / CH

* goose：`202607180005_access_log_cache_status.sql`
* `internal/model/analytics/node_access_log.go`、writer、list/scan、store 映射
* `internal/model/openflare_observability.go`、agent build records
* API `AccessLogView` + list 响应带 `cache_status`

### 前端

* types / 明细列表标签 / 详情字段
* 三态 helper：`resolveCacheOutcome(cache_status)`

---

## 4. 验证

* `go test ./internal/apps/agent/observability/ ./internal/repository/analytics/ ./internal/apps/openflare/agent/`
* `make swagger`（若 Handler 响应结构变更）
* `make code-check` / `make prettier`

---

## 5. 落地进度

* [x] log_format + protocol + agent parse
* [x] CH migration + 写入/读取
* [x] API + 前端明细展示
* [ ] 测试与提交
