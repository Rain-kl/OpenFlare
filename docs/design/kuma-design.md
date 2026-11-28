# Uptime Kuma 监控同步设计

OpenFlare 通过控制面内置的后台同步任务，经 Socket.IO 协议将启用状态的代理站点增量同步至外部 Uptime Kuma 实例，并以专属 `OpenFlare` 标签隔离作用域，确保只维护网关托管监控项而保留历史指标与用户手动创建的监控任务。

---

## 1. 架构与控制流

同步任务完全运行在 OpenFlare Server 控制面（`internal/apps/openflare/uptimekuma/`），不经过边缘节点：

```text
OpenFlare Server（每分钟 Cron 检查同步间隔）
  └─ 获取互斥锁，防止上轮同步未完成时并发重连
       └─ Socket.IO 握手与登录外部 Uptime Kuma
            ├─ 校验或自动创建专属标签 "OpenFlare"（默认色 #4f46e5）
            ├─ 等待接收完整 monitorList 事件后再执行比对
            └─ 仅对携带 "OpenFlare" 标签的监控执行 add / editMonitor / deleteMonitor
```

* **凭据收拢**：Uptime Kuma 连接地址与账号凭据保存在控制面系统配置中，边缘节点无需直接连接监控平台。
* **完整列表屏障**：必须收到 Socket.IO 推送的完整 `monitorList` 事件后才允许进入差分流程，避免列表未就绪导致误清理。

---

## 2. 标签隔离与差分同步状态机

1. **标签防污染隔离**：拉取 `monitorList` 后仅过滤带有 `OpenFlare` 标签的监控项；未携带该标签的任意既有监控项对同步器不可见，不会被编辑或删除。
2. **URL 提取规则**：对启用中的反向代理路由优先提取关联的具体域名（根据 `enable_https` 组装 `https://` 或 `http://`）；若路由仅绑定了通配符域名（如 `*.example.com`），跳过新增或更新并记录日志，同时保留既有监控不被误删。
3. **差分属性比对**：同名且带 `OpenFlare` 标签的监控已存在时，比对以下 5 项全局配置；任一字段变化触发 `editMonitor` 更新，完全一致则跳过，避免无意义写入：
   * 探测地址 `Url`
   * 探测周期 `Interval`（默认 `60s`）
   * 最大重试 `MaxRetries`
   * 重试间隔 `RetryInterval`（默认 `60s`）
   * 请求超时 `Timeout`（默认 `48s`）
4. **下线清理**：不再处于监控范围（禁用或删除）的站点若在 Kuma 中仍存在带 `OpenFlare` 标签的同名监控，则发送 `deleteMonitor` 清理。
