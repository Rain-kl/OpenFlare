# OpenFlare Edge 上报协议与观测数据模型

本文档定义 `openflare-edge` 与控制面之间的 `NodePayload` 心跳/WebSocket 上报契约、归一化入库流水线，以及 ClickHouse 与 PostgreSQL/SQLite（经 `internal/repository/logstore`）的表结构模型。传输分层与采集频次参见 [边缘可观测性与流量统计架构](./observability-design.md)。

---

## 1. 分层写入总览

```text
NodePayload (schema_version=2, engine="pingora")
  ├─ access_logs[] + buffered[].access_logs ─▶ logstore.AccessLogs ─▶ of_node_access_logs ─▶ of_access_log_hourly (CH MV)
  │                                        └─▶ TCP_INFO + NetIntel ─▶ netquality.passive ─▶ of_network_quality_samples
  ├─ host_metrics + buffered[].host_metrics  ─▶ logstore.Observability ─▶ of_node_metric_snapshots ─▶ of_node_metric_capacity_hourly (CH MV)
  ├─ edge_health + openresty_status/message  ─▶ 主库 of_nodes 最新态（of_node_edge_health 已移除）
  ├─ profile                                 ─▶ 主库 of_node_system_profiles（硬件清单 upsert）
  └─ health_events[]                         ─▶ 主库健康事件表（不进入分析明细库）
```

独立控制通道同时承载主动探测与 DNS 监控写入：

* 边缘探测任务结果（WebSocket `ping_result` / `POST /api/v1/agent/ping/result`）→ `of_node_ping_records`（任务定义在 `of_ping_tasks`）；
* SmartDNS 心跳与嵌入式 DNS 统计 → `of_dns_metric_snapshots`（via `logstore.Observability.InsertDNSMetricSnapshot`）。

---

## 2. 上报数据结构 (`internal/openflare/protocol/agent.go`)

### 2.1 顶层 `NodePayload`

HTTP 心跳（`POST /api/v1/agent/nodes/heartbeat`）与 WebSocket `status` 消息共用同一结构体：

| 字段 | 类型 | 语义 |
| :--- | :--- | :--- |
| `schema_version` | `int` | 协议版本号（当前为 `2`） |
| `node_id` / `name` / `ip` | `string` | 节点标识、名称与当前上报公网 IP |
| `version` / `ext_version` | `string` | `openflare-edge` 版本号 |
| `current_version` / `latest_config_version` | `string` | 当前正在服务的健康配置修订与最新观测修订 |
| `engine` / `contract_version` / `config_sync_version` | `string` / `int` | 运行引擎（`pingora`）、配置契约版本（`4`）与同步版本（`1`） |
| `edge_update_supported` / `edge_update_ready` | `bool` | 是否具备自更新条件及新版本就绪确认 |
| `last_error` | `string` | 最近配置应用或运行错误信息 |
| `openresty_status` / `openresty_message` | `string` | 沿用兼容字段名，保存边缘网关最新健康态（`healthy` / `unhealthy` / `unknown`）及说明，仅写主库 `of_nodes` |
| `profile` | `*NodeSystemProfile` | Komari 对齐主机静态/硬件画像（`of_node_system_profiles`） |
| `host_metrics` | `*NodeMetricSnapshot` | L3 Komari 对齐主机实时读数与累计计数器 |
| `edge_health` | `*NodeEdgeHealth` | L2 瞬时健康态与 `/proc/self/fd` 连接数（写 `of_nodes.openresty_connections`） |
| `access_logs` | `[]NodeAccessLog` | L1 增量请求访问明细（含 `TCP_INFO` 与 WAF 审计） |
| `buffered` | `[]BufferedObservabilityRecord` | 离线回放批次（包含历史窗 `host_metrics`、`edge_health`、`access_logs`） |
| `health_events` | `[]NodeHealthEvent` | 边缘健康状态变更事件 |

> [!NOTE]
> 旧版 `traffic_report`、`openresty_observation`、`snapshot` 与 `buffered_observability` 别名以及旧版心跳内嵌 `waf_ip_group_checksums` 字段均已从契约移除；WAF IP 组改为通过 `config_sync v1` 资源清单 `waf.ip-group.v1/<id>` 独立协商同步。

### 2.2 `profile`（Komari 对齐硬件画像）

写入主库 `of_node_system_profiles`，包含 `hostname`、`os_name`、`os_version`、`kernel_version`、`architecture`、`cpu_model`、`cpu_cores`、`cpu_physical_cores`、`virtualization`、`gpu_name`、`total_memory_bytes`、`swap_total_bytes`、`total_disk_bytes`、`ipv4`、`ipv6`、`probe_version`、`uptime_seconds` 与 `reported_at_unix`。

### 2.3 `host_metrics`（Komari 对齐主机指标）

所有网卡与磁盘字节字段上报 OS **累计计数器原值**，由控制面在查询时做相邻采样非负差分（节点重启计数器归零时当段记为 `0`），禁止边缘在本地折算为 24 小时增量或混同业务「已提供数据」：

| 协议字段 | 入库列 (`of_node_metric_snapshots`) | 含义 |
| :--- | :--- | :--- |
| `captured_at_unix` | `captured_at` | 采集时间戳（UTC） |
| `cpu_usage_percent` / `gpu_usage_percent` | `cpu` / `gpu` | 瞬时 CPU / GPU 利用率（%） |
| `memory_used_bytes` / `memory_total_bytes` | `ram` / `ram_total` | 内存当前已用量与总量（字节） |
| `swap_used_bytes` / `swap_total_bytes` | `swap` / `swap_total` | Swap 当前已用量与总量（字节） |
| `load1` / `load5` / `load15` / `temp` | `load` / `load5` / `load15` / `temp` | 1/5/15 分钟系统负载与温度 |
| `storage_used_bytes` / `storage_total_bytes` | `disk` / `disk_total` | 磁盘已用量与总量（字节） |
| `disk_read_bytes` / `disk_write_bytes` | `disk_read_bytes` / `disk_write_bytes` | 磁盘累计读/写字节 |
| `net_in` / `net_out` | `net_in` / `net_out` | 瞬时宿主机网卡入/出速率 |
| `net_total_down` / `net_total_up` | `net_total_down` / `net_total_up` | 宿主机网卡累计接收/发送总字节 |
| `process_count` / `tcp_connections` / `udp_connections` / `uptime_seconds` | `process` / `connections` / `connections_udp` / `uptime` | 系统进程数、TCP/UDP 连接数与运行秒数 |

### 2.4 `edge_health`（L2 瞬时态）

包含 `captured_at_unix`、`status`（`healthy`/`unhealthy`/`unknown`）、`message` 与 `connections`（当前已连接套接字数）。控制面规范化后统一写回主库 `of_nodes` 最新状态，已不再写入任何独立健康时序表。

### 2.5 `access_logs[]`（L1 业务明细与被动链路观测）

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `logged_at_unix` | `int64` | 请求完成时间戳（UTC 秒） |
| `remote_addr` | `string` | 下游客户端 IP（用于 UV 与 NetIntel 查询） |
| `host` | `string` | 请求 Host（用于 Zone 归属） |
| `path` | `string` | 请求路径及查询串（控制面常规截断至 `100` 字符；WAF 命中记录按攻击日志长度策略保留） |
| `user_agent` | `string` | User-Agent（入库截断至 `512` 字符） |
| `cache_status` | `string` | 缓存阶段：`HIT`、`MISS`、`BYPASS`、`EXPIRED`、`STALE`、`UPDATING`、`REVALIDATED`、`-` |
| `status_code` | `int` | 最终 HTTP 响应状态码 |
| `bytes_sent` | `int64` | 响应体交付字节数（业务口径「已提供数据」） |
| `request_length` | `int64` | 请求头与请求体估算接收字节数（业务口径「接收数据」） |
| `request_time_ms` | `int64` | 请求总耗时（毫秒） |
| `tcp_rtt_us` / `tcp_min_rtt_us` / `tcp_rtt_var_us` | `uint32` | 可选，内核 `TCP_INFO` 平滑 RTT、最小 RTT 与抖动（微秒） |
| `tcp_retrans` / `tcp_segs_out` | `uint32` | 可选，`TCP_INFO` 累计重传数与出向段数（用于被动重传率计算） |
| `tls_handshake_ms` | `uint32` | 可选，TLS 握手耗时（毫秒） |
| `waf` | `*WAFAudit` | 可选，拦截或仅观察命中时携带 `action`、`reason`、`flow_id`、`flow_node_id`、`attack_group_id`、`rules` 与截断请求预览；控制面拆分写入 `waf_action` 与 `waf_audit` |

控制面在 `internal/apps/openflare/agent/observability.go` 中自动补全 `id`（Snowflake UInt64）、`region`（GeoIP）与 `client_asn`（NetIntel），并在 `client_asn > 0 && tcp_rtt_us > 0` 时异步投递至 `netquality.RecordPassiveSampleInput`。

---

## 3. 核心表结构（ClickHouse / 业务主库同构）

> 在 ClickHouse 与 PostgreSQL/SQLite 双端均通过 `internal/infra/persistence/migrator/goose/` 维护同名列；PostgreSQL 对高频明细/指标表按月建立 `PARTITION BY RANGE`，ClickHouse 采用 `MergeTree` / `SummingMergeTree` / `AggregatingMergeTree` 与 `toYYYYMM` 分区及原生 TTL。

### 3.1 `of_node_access_logs`（L1 业务明细，默认 90 天保留）

* **ClickHouse 主键排序**：`ORDER BY (node_id, logged_at, host, status_code, remote_addr)`，TTL `90 DAY`。
* **核心列**：`id`、`node_id`、`logged_at`、`remote_addr`、`region`、`host`、`path`、`user_agent`、`cache_status`、`status_code`、`bytes_sent`、`request_length`、`request_time_ms`、`waf_action`、`waf_audit`、`client_asn`、`tcp_rtt_us`、`created_at`。
* **派生小时物化视图（仅 ClickHouse）**：`of_access_log_hourly`（`SummingMergeTree`，按 `(node_id, hour, host)` 汇总 `request_count`、`error_count`、`bytes_sent`、`request_length`），供 24 小时以上趋势加速查询；PostgreSQL/SQLite 关闭物化视图并直接从原表实时聚合。

### 3.2 `of_node_metric_snapshots`（L3 主机快照，默认 30 天保留）

* **ClickHouse 主键排序**：`ORDER BY (node_id, captured_at, id)`，TTL `30 DAY`。
* **核心列**：`id`、`node_id`、`captured_at`、`cpu`、`gpu`、`ram`、`ram_total`、`swap`、`swap_total`、`load`、`load5`、`load15`、`temp`、`disk`、`disk_total`、`disk_read_bytes`、`disk_write_bytes`、`net_in`、`net_out`、`net_total_up`、`net_total_down`、`process`、`connections`、`connections_udp`、`uptime`、`created_at`。
* **派生小时物化视图（仅 ClickHouse）**：`of_node_metric_capacity_hourly`（`AggregatingMergeTree`，保存 CPU/内存加和与计数，以及 `net_total_down`、`net_total_up`、`disk_read_bytes`、`disk_write_bytes` 的小时 `min`/`max`）。

### 3.3 `of_node_ping_records` 与 `of_dns_metric_snapshots`

* **`of_node_ping_records`**：记录 `id`、`node_id`、`task_id`、`recorded_at`、`value`（毫秒，失败通常为 `-1`）、`created_at`；ClickHouse 按 `(node_id, task_id, recorded_at, id)` 排序，TTL `30 DAY`。
* **`of_dns_metric_snapshots`**：记录 `id`、`instance_id`、`captured_at`、`queries_total`、`queries_success`、`queries_nxdomain`、`queries_servfail`、`queries_ratelimited`、`qps`、`latency_p50_ms`、`latency_p95_ms`、`latency_p99_ms`、`cache_hit_ratio`、`snapshot_version`、`cpu`、`ram_percent`、`created_at`；ClickHouse 按 `(instance_id, captured_at, id)` 排序，TTL `30 DAY`。

### 3.4 `of_network_quality_samples`（网络质量分桶样本，默认 30 天保留）

* **ClickHouse 主键排序**：`ORDER BY (node_id, client_asn, captured_at, id)`，TTL `30 DAY`。
* **核心列**：`id`、`node_id`、`node_asn`、`client_asn`、`client_country`、`client_region`、`ip_family`、`source`（`tcp_passive`/`tcp_active`/`icmp_active`/`traceroute`）、`sample_count`、`rtt_min_ms`、`rtt_p50_ms`、`rtt_p95_ms`、`retrans_rate`、`loss_rate`、`hop_count`、`stats_version`、`rtt_hist`（Base64 `LogHistogram64`）、`prefix_sketch`（`PrefixSketch`）、`distinct_prefix_count`、`client_lat`、`client_lon`、`geo_accuracy_km`、`geo_source`、`captured_at`、`created_at`。

### 3.5 已移除的历史观测表

以下旧版 OpenResty / FRP / 冗余时序表均已通过 Goose 迁移删除，不再存在写入或查询路径：

* `of_node_edge_health`（PG/SQLite 迁移 `202611270006`、ClickHouse 迁移 `202611270003` 移除，最新连接数合并至 `of_nodes.openresty_connections`）
* `of_node_obs_frps`、`of_node_obs_frpc`（ClickHouse 迁移 `202611230001` 移除）
* `of_node_request_reports`、`of_node_traffic_hourly`、`of_node_obs_openresty`、`of_node_openresty_hourly`
