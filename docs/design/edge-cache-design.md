# 边缘缓存策略设计

OpenFlare Edge（`edge/src/proxy/cache.rs`）通过 `pingap-cache` 接入 TinyUFO 内存缓存及可选磁盘持久化存储，结合节点性能策略开关与站点规则（Proxy Route）策略实现可控的 HTTP 响应缓存。访问日志中的 `cache_status` 字段定义见 [OpenFlare Edge 上报协议与观测数据模型](./observability-data-model.md#25-access_logsl1-业务明细与被动链路观测)。

---

## 1. 双层开关与缓存作用域

进入边缘缓存链路须同时满足以下控制条件：

1. **节点性能策略（`cache_enabled`）**：控制 `EdgeCache` 存储是否启用于当前节点；关闭时整台节点跳过缓存查写。
2. **站点路由开关（`route.cache_enabled`）**：控制该反向代理路由是否参与缓存；关闭时直接回源。
3. **功能互斥边界**：启用站点认证（`site_auth_ttl_hours != 0`）或 HTTP Basic Auth 的站点跳过响应缓存；Pages 静态资源直接由 `edge/src/pages/` 凭 `ETag`/`Range` 本地服务，不经过代理缓存层。

---

## 2. 两阶段缓存判定（`edge/src/proxy/cache.rs`）

### 2.1 请求侧准入（Stage A）

收到请求后，`EdgeCache::enable` 执行以下准入检查，任一条件不满足则跳过缓存查写（最终记录为 `BYPASS` 或 `-`）：

* **方法限制**：仅缓存 `GET` 请求；`HEAD`、`POST` 等非 `GET` 方法直接旁路。
* **敏感/分片/升级请求头旁路**：携带 `Authorization`、`Upgrade`（如 WebSocket）或 `Range` 请求头的请求直接跳过缓存，避免凭据内容错配或分片状态污染。
* **路由缓存策略（`cache_policy`）匹配**：

| `cache_policy` | 含义 | 默认 | 兼容说明 |
| :--- | :--- | :--- | :--- |
| **`static`** | 仅匹配内置静态扩展名列表（不含 `html`、`htm`、`json`） | **是** | 大小写不敏感匹配 |
| **`all`** | 不限制扩展名与路径（高级选项，须源站正确声明 `Cache-Control`） | 否 | 历史路由配置 `url` 在 API 读写与快照编译时规范化为 `all` |
| **`suffix`** | 自定义扩展名后缀列表（`cache_rules`） | 否 | 至少配置 1 项扩展名 |
| **`path_prefix`** | 自定义路径前缀列表（`cache_rules`） | 否 | 以 `/` 开头 |
| **`path_exact`** | 自定义精确路径列表（`cache_rules`） | 否 | 以 `/` 开头 |

内置 `static` 默认扩展名集合（来自 `internal/openflare/edgeconfig/types.go` / `edge/src/protocol/config.rs`）：

```text
css js mjs map
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

> [!NOTE]
> 请求携带常规站点 Cookie 时，请求侧**不会**单纯因 Cookie 跳过 `static` 等可缓存路径；个性化防泄密由响应侧的 `Set-Cookie` 与源站 `Cache-Control` 闸门保证。

### 2.2 响应侧可缓存性与默认 TTL（Stage B）

请求准入后，回源响应在 `EdgeCache::response` 经过严格检验才写入 TinyUFO：

* **禁止入库条件**：响应包含 `Set-Cookie`、包含 `Vary: *` 或源站声明 `Cache-Control: private` / `no-store` / `no-cache` 等不可共享指令时，直接透传响应但不写入缓存（防止会话泄露）。
* **缓存键隔离与 `Vary` 维度**：主缓存键对 `{snapshot_digest}|{max_object_bytes}|{scheme}|{host}|{request_uri}` 执行哈希（含查询串，日志与存储路径只留不透明哈希 ID），并通过 `Vary` 头列举请求头参与二次变体维度区分。
* **回源并发锁**：通过全局 `CacheLock`（60 秒超时、配合性能策略 `cache_lock_wait_timeout` 与 `cache_max_lock_retries`）合并回源，抑制缓存击穿；单对象上限同时受性能策略 `cache_max_object_mib` 与存储容量限制（磁盘模式下受节点内存预算 1/4 上限约束）。
* **状态码默认 Edge TTL**：当源站未返回显式 freshness 头（`Cache-Control` / `Expires`）时，网关应用以下默认 TTL，并可选支持 `stale_updating` / `stale_on_error`（默认 60 秒陈旧服务窗口）：

| 响应状态码 | 默认 Edge TTL |
| :--- | :--- |
| `200`, `206`, `301` | `7200s`（120 分钟） |
| `302`, `303` | `1200s`（20 分钟） |
| `404`, `410` | `180s`（3 分钟） |

---

## 3. 代码与配置实现面

| 组件 | 路径 |
| :--- | :--- |
| 边缘缓存键、准入、TinyUFO 内存/磁盘与后台清理 | `edge/src/proxy/cache.rs`、`edge/src/proxy/mod.rs` |
| 性能策略存储参数与启动校验 | `edge/src/runtime/performance.rs` |
| 路由缓存策略校验与 `url → all` 规范化 | `internal/apps/openflare/proxy_route/helpers.go` |
| 快照编译默认扩展名注入 | `internal/openflare/edgeconfig/`、`internal/apps/openflare/config_version/snapshot.go` |
| 控制台缓存策略面板 | `frontend/app/(main)/proxy-routes/detail/components/cache-section.tsx` |
