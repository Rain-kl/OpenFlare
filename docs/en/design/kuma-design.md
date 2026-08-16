# Uptime Kuma Sync Design

You will learn: the design background of the OpenFlare × Uptime Kuma monitoring integration, the control-flow design based on the Socket.IO protocol, the anti-pollution model centered on tag isolation, and the differential incremental sync state machine.

---

## Requirements Analysis

In a multi-node gateway architecture, monitoring system state and reverse proxy route state are usually disconnected:
1. **High entry overhead**: every time the gateway control plane adds or decommissions a site, the admin must re-configure the corresponding probe address and alert policy in the monitoring system (e.g. Uptime Kuma).
2. **Data inconsistency**: when a proxy route domain changes or switches to HTTPS, monitoring parameters are easily left un-updated, causing false positives or missed alerts.
3. **Environment pollution risk**: a full "delete-recreate" sync in monitoring would wipe historical statistics and SLA curves, and would also affect other monitor tasks the user configured manually on the instance that are unrelated to the gateway.

To address these, OpenFlare introduces a **Uptime Kuma auto-monitoring sync mechanism** based on the client/server model, achieving strongly consistent, low-overhead, zero-pollution synchronization between gateway site route definitions and the availability monitoring system.

---

## Core Architecture

The Uptime Kuma sync subsystem runs entirely in the **Server control plane** background scheduler.

```text
  [ OpenFlare Control Plane / DB ]                    [ Uptime Kuma Instance ]
              │                                             │
      1. Scheduled Cron trigger (Job)                       │
              │                                             │
      2. Read proxy routes & options config                 │
              │                                             │
      3. Connect to Socket.IO <──── 4. Socket.IO handshake & login ────┤
              │                                             │
              ├────── 5. Validate / create "OpenFlare" tag ──►│
              ├────── 6. Compare site attrs vs Kuma monitor list ─►│
              │                                             │
              └────── 7. Execute differential ops (add / edit / delete) ─►│
```

The sync subsystem does not pass through the data-plane Agent nodes; the Server talks directly to Uptime Kuma's exposed Socket.IO endpoint. This reduces edge node network overhead and keeps auth credentials (Kuma username/password) safely inside the control plane.

---

## Tag Isolation and Anti-Pollution Design

To run safely in a shared Uptime Kuma instance without disturbing manually created monitors, a **dedicated tag isolation mechanism** is used:

1. **`OpenFlare`-specific tag**:
   * On first connect, the sync routine calls `getTags` to fetch all tags in the instance.
   * It checks whether a tag named `OpenFlare` exists (default color indigo `#4f46e5`). If not, it creates it automatically via the `addTag` API.
2. **Filtered scope**:
   * After fetching Uptime Kuma's monitor list (`monitorList`), the sync task only keeps monitors **tagged with `OpenFlare`**.
   * All modification comparisons (`editMonitor`) and offline cleanups (`deleteMonitor`) operate **only within this filtered subset**. Any monitor not bound with the `OpenFlare` tag is "invisible" to the sync routine — perfect anti-pollution isolation.

---

## Differential Sync State Machine

On each run, the sync routine computes a diff between OpenFlare's local config and Uptime Kuma's data, then executes different Socket.IO events based on the comparison:

```mermaid
stateDiagram-v2
    [*] --> 检查站点状态与监控范围
    
    state "检查监控范围" as Scope {
        [*] --> 校验站点是否启用并且在 Scope 内
        校验站点是否启用并且在 Scope 内 --> 在Scope内 : 是
        校验站点是否启用并且在 Scope 内 --> 不在Scope内 : 否
    }

    不在Scope内 --> 检查Kuma中是否存在同名且带标签的监控
    检查Kuma中是否存在同名且带标签的监控 --> 执行清理 : 存在
    检查Kuma中是否存在同名且带标签的监控 --> 忽略 : 不存在

    在Scope内 --> 检查Kuma中是否存在同名监控
    
    state "比对属性" as Compare {
        [*] --> 检查是否存在
        检查是否存在 --> 新建监控项 : 否
        检查是否存在 --> 比对元数据 : 是
        比对元数据 --> 属性一致 : 匹配
        比对元数据 --> 属性不一致 : 不匹配
    }

    新建监控项 --> 发送add指令并绑定Tag
    属性不一致 --> 发送editMonitor指令
    属性一致 --> 忽略

    执行清理 --> 发送deleteMonitor指令
    忽略 --> [*]
```

### 1. Monitor URL Normalization
A site route in OpenFlare can configure multiple domains; the sync routine automatically extracts the primary domain and assembles a standard `http://` or `https://` prefix based on whether HTTPS is enabled.

### 2. Compared Attribute Set
If a same-named, tagged monitor already exists, the sync routine compares the following 5 key fields against the current gateway global option. Any mismatch triggers an update:
* **URL**: `Url`
* **Probe interval**: `Interval` (default 60s)
* **Max retries**: `MaxRetries`
* **Retry interval**: `RetryInterval` (default 60s)
* **Request timeout**: `Timeout` (default 48s)

---

## Scheduler and High-Concurrency Protection

1. **Cron-based single-thread execution**:
   * The Server periodically (every 1 minute) probes via a background Cron Job whether the configured sync interval (`UptimeKumaSyncInterval`) is reached.
   * The task uses mutex locking internally. If a previous sync request is still running due to network latency, the next schedule is skipped automatically, preventing concurrent Socket.IO connections from DDOS-ing the Uptime Kuma instance.
2. **WebSocket state listening**:
   * The sync routine uses Socket.IO's event listener; after the connection is established, it only proceeds to the differential algorithm once the full `monitorList` event list push is received, avoiding monitor deletion caused by incomplete data loading.
