# 边缘观测数据传输模型（现行目标版）

> **本文是「Agent ↔ Server 观测数据怎么传」的最新权威说明。**  
> 读完应能回答：传什么、从哪采、多久采一次、Server 怎么存、产品指标从哪查。  
> 协议字段与 DDL 细节另见 [观测上报协议与表结构](./observability-data-model.md)；问题背景见 [边缘可观测与业务流量统计](./observability-design.md)。

---

## 0. 先记住三层（不要混）

| 层 | 回答的问题 | 唯一数据来源 | 产品例子 |
| --- | --- | --- | --- |
| **L1 业务交付** | 提供了多少数据？多少请求？ | **access.log 明细** | 已提供数据、请求数、UV、状态码、Top 域名 |
| **L2 边缘健康** | OpenResty 活着吗？现在多少连接？ | **本机 `/openflare/observability`** | 节点健康、当前连接 |
| **L3 宿主机资源** | CPU/内存/磁盘/网卡怎样？ | **操作系统读数** | 容量趋势、宿主机网卡 |

**三层互不对账。**  
「已提供数据」≠「当前连接」≠「宿主机网卡出站」。

---

## 1. 总览：谁采集、谁上报、谁聚合

```text
┌─────────────────────────────────────────────────────────────┐
│ 边缘节点                                                      │
│                                                               │
│  访客请求 ──► OpenResty                                        │
│                 │                                              │
│                 ├─ access.log（每请求一行）  ←── L1 采集点      │
│                 │                                              │
│                 └─ 连接状态（进程内维护）                         │
│                        │                                       │
│                        ▼                                       │
│              GET /openflare/observability  ←── L2 读快照       │
│              （不扫日志、不重算业务量）                            │
│                                                               │
│  操作系统 /proc 等  ────────────────────── L3 读快照            │
│                                                               │
│              ┌────────── Agent ──────────┐                     │
│              │ 默认每 3s 组一包 NodePayload  │                     │
│              │  · tail access.log 增量    │                     │
│              │  · GET 本机 observability  │                     │
│              │  · 读 host_metrics         │                     │
│              └────────────┬──────────────┘                     │
└─────────────────────────────│──────────────────────────────────┘
                              │ HTTP 心跳 或 WebSocket status
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ Server（控制面）                                               │
│  · 明细 → ClickHouse of_node_access_logs                      │
│  · 健康 → 节点最新态 + of_node_edge_health                      │
│  · 主机 → of_node_metric_snapshots                            │
│  · 业务趋势 / Zone 统计 = 只对 access_logs 做 sum/count/uniq   │
└─────────────────────────────────────────────────────────────┘
```

| 角色 | 做什么 | 不做什么 |
| --- | --- | --- |
| OpenResty | 写 access.log；维护连接数 | 不向控制面直接上报 |
| Agent | **采集事实并上报** | **不算** UV/TopN/24h 已提供数据 |
| Server | 入库 + **聚合解释** | 不信任边缘业务预汇总 |

---

## 2. 采集频率（默认）

| 动作 | 默认频率 | 配置 |
| --- | --- | --- |
| Agent → Server 上报 | **每 3 秒** 一次完整 payload | `heartbeat_interval` / 控制面 `agent_heartbeat_interval`（毫秒，默认 `3000`） |
| 组包时 tail access.log | **随上报**（两次上报之间的新行） | 同上 |
| 组包时 GET `/openflare/observability` | **随上报**（读**当前**连接快照） | 同上 |
| 组包时读主机指标 | **随上报** | 同上 |
| OpenResty 写 access.log | **每个请求结束时** 1 行 | 与心跳无关 |
| 连接数在进程内更新 | **连接变化时**（内核维护） | 与心跳无关 |
| 离线补传窗口 | 默认保留约 **60 分钟** | `observability_replay_minutes` |
| 节点离线判定 | 约 **60 秒** 无成功心跳 | `node_offline_threshold`（默认 `60000` 毫秒） |

**说明：**

- Agent **没有**单独的「采样时钟」；**采样点 = 上报点**（默认 3s）。  
- access.log 是「请求级连续写入」；Agent 只是周期性 **搬运增量行**。  
- `/openflare/observability` **不是**「被调用才开始统计业务」；对连接而言是 **读 Nginx 已有瞬时值**。

传输通道：

- **HTTP 心跳**：按间隔 POST 整包。  
- **WebSocket**：连通后按同一间隔发 `status` 消息（内容同构）；此时不再走 HTTP 心跳双发。

---

## 3. Agent → Server 数据包（NodePayload v2）

### 3.1 结构骨架

```json
{
  "schema_version": 2,
  "node_id": "n_01hxyz",
  "name": "edge-shanghai-1",
  "ip": "203.0.113.10",
  "version": "3.4.0",
  "ext_version": "",
  "current_version": "20260718-abc",
  "last_error": "",
  "profile": { },
  "host_metrics": { },
  "edge_health": { },
  "access_logs": [ ],
  "buffered": [ ],
  "health_events": [ ],
  "waf_ip_group_checksums": { }
}
```

| 字段 | 层 | 含义 |
| --- | --- | --- |
| 身份/版本/last_error | 控制 | 节点是谁、跑什么版本 |
| `profile` | 低频概况 | 主机名、核数等（变化才报） |
| `access_logs` | **L1** | 访问明细增量 |
| `edge_health` | **L2** | OpenResty 健康 + 当前连接 |
| `host_metrics` | **L3** | CPU/内存/磁盘/网卡读数 |
| `buffered` | 补传 | 离线期间攒的事实批次 |
| `health_events` | 事件 | 如 openresty_unhealthy |
| `waf_ip_group_checksums` | 同步 | 非观测湖 |

**协议已删除（无兼容层，旧 Agent 必须升级）：**

- `traffic_report`  
- `openresty_observation`（含 rx/tx）  
- `snapshot` / `buffered_observability`  
- 业务含义的 openresty 吞吐字段  

---

## 4. L1 业务：access_logs

### 4.1 采集从哪里来

| 步骤 | 位置 | 说明 |
| --- | --- | --- |
| 1 | OpenResty `log_format openflare_json` | 每请求写一行 JSON 到 `access_log_path` |
| 2 | Agent 按文件 offset **tail 增量** | 两次心跳之间的新行 |
| 3 | 解析后放入 `access_logs[]` | 可截断过长 path；**不做 sum/count** |

日志格式（OpenResty 变量）：

```text
ts            ← $time_iso8601
host          ← $host
path          ← $request_uri
remote_addr   ← $remote_addr
status        ← $status
request_time  ← $request_time
bytes_sent    ← $body_bytes_sent     【已提供数据 = 响应体字节】
request_length← $request_length      【接收数据】
user_agent    ← $http_user_agent
cache_status  ← $upstream_cache_status  【缓存状态；UI 可推导命中/回源/未缓存】
```

观测端口请求 **不写** 业务 access.log（独立 server `access_log off`）。

### 4.2 上报示例

```json
"access_logs": [
  {
    "logged_at_unix": 1721289601,
    "remote_addr": "198.51.100.20",
    "host": "www.example.com",
    "path": "/api/v1/ping",
    "status_code": 200,
    "bytes_sent": 1024,
    "request_length": 128,
    "request_time_ms": 15,
    "user_agent": "curl/8.0",
    "cache_status": "MISS"
  },
  {
    "logged_at_unix": 1721289602,
    "remote_addr": "198.51.100.21",
    "host": "www.example.com",
    "path": "/index.html",
    "status_code": 200,
    "bytes_sent": 8192,
    "request_length": 300,
    "request_time_ms": 8,
    "user_agent": "Mozilla/5.0",
    "cache_status": "HIT"
  }
]
```

| 字段 | 解释 |
| --- | --- |
| `bytes_sent` | **已提供数据**（单请求）；全局/Zone 合计 = Server `sum` |
| `request_length` | **接收数据**（单请求） |
| `logged_at_unix` | 请求完成时间（业务时间轴） |
| `host` | 用于 Zone 域名过滤 |
| `cache_status` | `$upstream_cache_status` 原样；详情/列表可推导三态（命中/回源/未缓存）；**不上报** upstream 地址 |
| 无 `region` | **Server 入库时** GeoIP 写入 |

### 4.3 Server 如何用（产品指标）

| 产品指标 | 算法（仅 L1） |
| --- | --- |
| 已提供数据 | `sum(bytes_sent)` |
| 接收数据 | `sum(request_length)` |
| 请求数 | `count()` |
| UV | `uniqExact(remote_addr)` |
| 状态码分布 | `group by status_code` |
| Top 域名 | `group by host` |
| Zone 页 | 同上 + `host IN (该 Zone 域名)` |
| 看板业务区 | 同上，全局或 Top 过滤 |

落库表：`of_node_access_logs`（可选 Server 侧 `of_access_log_hourly` 加速，**Agent 不写**）。

### 4.4 频率再强调

```text
请求发生 ──立即──► 写 access.log
Agent 每 3s ──搬运──► 这 3s 内新行（可能 0 行，也可能很多行）
Server ──立即/批量──► CH
```

业务量正确性 **不依赖** 3s 对齐；3s 只影响「明细到达控制面的延迟」和单包条数。

---

## 5. L2 健康：edge_health 与 `/openflare/observability`

### 5.1 本机监测口（合并后目标）

**只保留一个接口：**

```http
GET http://127.0.0.1:{openresty_observability_port}/openflare/observability
```

默认端口：**18081**（`openresty_observability_port`）。

**职责：** 回答「OpenResty 此刻怎样」，**不**回答业务已提供多少数据。

#### 返回示例（目标 JSON）

```json
{
  "ok": true,
  "captured_at_unix": 1721289600,
  "connections": {
    "active": 42,
    "reading": 0,
    "writing": 1,
    "waiting": 41
  }
}
```

| 字段 | 是否瞬时 | 从哪来 | 说明 |
| --- | --- | --- | --- |
| `ok` | 当次探测 | 能返回 200 即 true | 探活 |
| `captured_at_unix` | 采样时刻 | `ngx.time()` | 与上报对齐 |
| `connections.active` | **瞬时** | Nginx 连接状态（原 stub_status Active） | 当前活跃连接 |
| `reading` / `writing` / `waiting` | **瞬时** | 同上细分 | 可选但建议带 |

**不返回（已从目标模型删除）：**

| 旧字段 | 原因 |
| --- | --- |
| `request_count` / `error_count` / UV / status_codes / top_domains | 业务窗汇总，改由 access log |
| `openresty_rx_bytes` / `openresty_tx_bytes` | 与已提供/接收数据重复且易错 |
| `source_countries` | 从未实现；国家走 Server GeoIP |
| `server.accepts/handled/requests` | 进程累计 counter，易与业务请求混淆；主路径不收录 |

**`/openflare/stub_status`：** 合并进上述 JSON 后 **删除**（过渡期可双挂，Agent 只打合并口）。

### 5.2 采集机制（读快照，不是「调用才开始统计业务」）

```text
Nginx 在连接建立/释放时维护 Active connections 等
        │
Agent GET /openflare/observability
        │
只读取「当前值」拼 JSON 返回
```

- **不是** GET 一次才去扫 access.log。  
- **不是** 60 秒业务均值。  
- 是 **瞬时 gauge 快照**。

### 5.3 上报示例（装进 NodePayload）

```json
"edge_health": {
  "captured_at_unix": 1721289600,
  "status": "healthy",
  "message": "",
  "connections": 42
}
```

| 字段 | 来源 |
| --- | --- |
| `status` / `message` | Agent 健康探测（配置校验/进程等，可与观测口 `ok` 配合）；须与顶层 `openresty_status` / `openresty_message` 对齐 |
| `connections` | 观测口 `connections.active` |

**落库拆分（权威源）：**

| 内容 | 写入 |
| --- | --- |
| 最新 `status` + `message` | **PG 节点表**（UI / 列表 / 告警） |
| 时序 `status` + `connections` | **CH `of_node_edge_health`**（**无 message**） |

---

## 6. L3 主机：host_metrics

### 6.1 采集从哪里来

Agent 读本机（如 `/proc`、磁盘统计等），**每次组包时读一次**。

| 字段 | 语义 | 说明 |
| --- | --- | --- |
| `cpu_usage_percent` | 瞬时 | 当前 CPU% |
| `memory_*` / `storage_*` | 瞬时用量/总量 | 占用率在 Server 或展示层算 |
| `disk_read_bytes` / `disk_write_bytes` | **累计 counter** | 内核累计 IO |
| `network_rx_bytes` / `network_tx_bytes` | **累计 counter** | **宿主机网卡**，不是已提供数据 |

### 6.2 上报示例

```json
"host_metrics": {
  "captured_at_unix": 1721289600,
  "cpu_usage_percent": 12.5,
  "memory_used_bytes": 4294967296,
  "memory_total_bytes": 16106127360,
  "storage_used_bytes": 50000000000,
  "storage_total_bytes": 107374182400,
  "disk_read_bytes": 9000000000,
  "disk_write_bytes": 12000000000,
  "network_rx_bytes": 500000000000,
  "network_tx_bytes": 800000000000
}
```

### 6.3 Server 如何处理累计字段

```text
存原值时间序列
展示「这段时间网卡出站」时：
  delta = 本次 - 上次
  若 delta < 0 → 视为重启/计数器归零，本段增量记 0，从新基线继续
  若 delta >= 0 → 记入该时段增量
```

- Agent **上报原值**，不在边缘算 24h 总量。  
- **禁止** 对累计原值做 `sum` 当业务量。  
- 文案必须是 **「宿主机网卡」**，禁止叫「已提供数据 / OpenResty 出站」。

落库：`of_node_metric_snapshots`（可选 capacity hourly MV）。

---

## 7. 一次完整上报示例（拼起来）

```json
{
  "schema_version": 2,
  "node_id": "n_01hxyz",
  "name": "edge-shanghai-1",
  "ip": "203.0.113.10",
  "version": "3.4.0",
  "ext_version": "",
  "current_version": "20260718-abc",
  "last_error": "",
  "host_metrics": {
    "captured_at_unix": 1721289600,
    "cpu_usage_percent": 12.5,
    "memory_used_bytes": 4294967296,
    "memory_total_bytes": 16106127360,
    "storage_used_bytes": 50000000000,
    "storage_total_bytes": 107374182400,
    "disk_read_bytes": 9000000000,
    "disk_write_bytes": 12000000000,
    "network_rx_bytes": 500000000000,
    "network_tx_bytes": 800000000000
  },
  "edge_health": {
    "captured_at_unix": 1721289600,
    "status": "healthy",
    "message": "",
    "connections": 42
  },
  "access_logs": [
    {
      "logged_at_unix": 1721289595,
      "remote_addr": "198.51.100.20",
      "host": "www.example.com",
      "path": "/",
      "status_code": 200,
      "bytes_sent": 4096,
      "request_length": 200,
      "request_time_ms": 12
    }
  ],
  "buffered": [],
  "health_events": [],
  "waf_ip_group_checksums": {
    "1": "d41d8cd98f00b204e9800998ecf8427e"
  }
}
```

**Server 落库示意：**

| payload 块 | 写入 |
| --- | --- |
| `access_logs[0]` | CH 一行，`bytes_sent=4096`，`region` 由 GeoIP 填 |
| `edge_health` | 节点 `openresty_status=healthy`，connections=42 |
| `host_metrics` | CH metric 一行累计/瞬时字段 |

**产品查询示意（24h）：**

- 已提供数据 = 该节点（或全局）日志 `sum(bytes_sent)`  
- 当前连接 = 最新 `edge_health.connections`  
- 宿主机网卡出站 = metric 上 `network_tx` 非负差分之和  

三者数字 **不必相等**。

---

## 8. 离线补传 `buffered`

Agent 上报失败时，把 **同一类事实** 按窗口缓存在本地（默认约 60 分钟），恢复后塞进 `buffered[]`：

```json
"buffered": [
  {
    "captured_at_unix": 1721289500,
    "host_metrics": { },
    "edge_health": { },
    "access_logs": [ ]
  }
]
```

- 只装事实，不装旧 TrafficReport。  
- Server 处理逻辑与主字段相同。

---

## 9. 端到端时序（默认 3s）

```text
t=0.0s   访客请求完成 → 写 access.log 一行；连接数可能变化
t=0.1s   又一请求 → 又一行 log
…
t=3s     Agent 心跳：
           · 读走 2 行 access_logs
           · GET observability → connections=42
           · 读 host_metrics
           · 发给 Server
t=3s+    Server 入库；看板/Zone 查询时聚合日志
t=6s     下一轮…
```

---

## 10. 旧模型对照（帮助消歧）

| 旧做法 | 新模型 |
| --- | --- |
| Lua dict 60s 窗 request_count + Agent 10s 拉 + Server sum | **删除**；请求数 = 日志 count |
| openresty_tx 当「出站」 | **删除**；已提供数据 = `sum(bytes_sent)` |
| 两个口 observability + stub_status | **合并为一个** observability，只返回连接/探活 |
| TrafficReport 预聚合 | **删除**；协议与 API 均无此路径 |
| 业务与网卡混称「流量」 | **分文案、分 API、分表** |
| 健康 status/message | **PG 最新态权威**；CH 仅 status+连接时序 |

---

## 11. 配置与实现索引

| 项 | 位置/键 |
| --- | --- |
| 心跳间隔 | Agent `heartbeat_interval`；控制面 `agent_heartbeat_interval`（默认 3000ms） |
| 离线阈值 | 控制面 `node_offline_threshold`（默认 60000ms） |
| 观测端口 | `openresty_observability_port`（默认 18081） |
| access.log 路径 | `access_log_path` |
| 补传分钟数 | `observability_replay_minutes`（默认 60） |
| 协议类型 | `pkg/protocol/agent.go`（落地时按 v2 演进） |
| 表结构 DDL | [observability-data-model.md](./observability-data-model.md) |

---

## 12. 修订记录

| 日期 | 说明 |
| --- | --- |
| 2026-07-18 | 初稿：作为「最新传输模型」单页说明——三层、频率、示例 JSON、采集来源、与旧模型对照 |
| 2026-07-18 | 默认上报间隔 3s；离线阈值 60s；补传窗口 60 分钟 |
| 2026-07-18 | M5：edge_health 表、access_log_hourly、废弃 request_reports/obs_openresty 吞吐表 |
| 2026-07-18 | 无兼容层：删除「兼容期可忽略」表述；健康 message 仅 PG、CH 无 message |
