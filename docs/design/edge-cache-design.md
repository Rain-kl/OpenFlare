# 边缘缓存策略设计

你会学到：OpenFlare 边缘网关缓存如何在「该缓存」与「不该缓存」之间对齐 Cloudflare 默认闭环：请求 eligible（扩展名/策略）× 响应可共享缓存（源站 `Cache-Control` / `Expires` / `Set-Cookie`），以及与过往过严请求旁路的差异。

本设计是 [系统架构](./architecture.md) 中「基础缓存」的产品化专章；访问日志中的缓存结果见 [观测数据模型 §3.5.1](./observability-data-model.md)。

---

## 1. 目标与非目标

### 1.1 目标

* **开箱接近 CF 默认**：路由开启缓存后，**默认只缓存静态扩展名**，不默认缓存 HTML；**不因请求会话 Cookie / Authorization / 客户端 Cache-Control 一律 BYPASS**。
* **该缓存的能命中**：带登录 Cookie 的用户访问 `/_app/**/*.js` 等静态资源可出现 `MISS` → `HIT`。
* **不该缓存的仍挡住**：策略不 eligible（等价 CF `DYNAMIC`）；源站 `private` / `no-store`；响应带 **`Set-Cookie` 不入库**（对齐 CF OCC 默认）；`all` 为高级选项并文档警示。
* **无源站 freshness 时有默认 Edge TTL**：对齐 CF 按状态码的默认 TTL（见 §3.5）。
* **可观测一致**：继续依赖边缘上报的 `cache_status` 明细。
* **兼容存量**：旧路由 `cache_policy=url` 映射为 `all`；策略枚举与迁移规则保持 [§5](#5-兼容与迁移)。

### 1.2 非目标（后续迭代）

* Cache Rules 表达式引擎  
* 忽略源站 `Cache-Control` 的强制 Edge TTL（CF Cache Rules「Ignore cache-control」）  
* Purge（按 URL/前缀/全站）  
* 浏览器 TTL 改写、客户端 `CF-Cache-Status` 响应头  
* 完整 RFC 条件：`Authorization` 仅当响应含 `public`/`s-maxage`/`must-revalidate` 才缓存  
* HEAD 转 GET 再缓存  
* 命中率看板  

---

## 2. Cloudflare 判定闭环（对齐基准）

CF 默认是 **两段决策**，**不是**「请求带 Cookie 就不缓存」。

### 2.1 阶段 A — 请求时 Eligible

| 条件 | CF 结果 |
| --- | --- |
| 非 GET | 默认不缓存 |
| 扩展名不在默认可缓存表，且无 Rules 强制 Eligible | **`DYNAMIC`**（不查缓存） |
| 扩展名在默认表，或 Rules Eligible | 继续阶段 B |
| **请求 Cookie** | **默认不影响** |
| Cache Rules Bypass | `DYNAMIC` |

CF 默认可缓存扩展名按 **扩展名** 而非 MIME；**默认不缓存 HTML / JSON**。

### 2.2 阶段 B — 响应是否可入库（OCC on，Free/Pro/Biz 默认）

| 条件 | 结果 |
| --- | --- |
| `Cache-Control: no-store` / `private` | 不入库 |
| `public` + `max-age>0`，或未来 `Expires` | 可缓存 |
| 无 Cache-Control / Expires | 按状态码 **默认 Edge TTL** 仍可缓存（如 200 → 120m） |
| 响应 **`Set-Cookie`**（默认缓存级别 + OCC） | **不入库**，状态倾向 **BYPASS** |
| 请求 `Authorization` | 仅当响应另有 `public` / `s-maxage` / `must-revalidate` 才可缓存 |

### 2.3 状态语义（对照观测）

| CF | 含义 | OpenFlare `cache_status` |
| --- | --- | --- |
| HIT / STALE / UPDATING / REVALIDATED | 命中类 | 同名或等价 |
| MISS / EXPIRED | 回源取内容 | 同名 |
| BYPASS | 请求时 eligible，响应不可缓存 | `BYPASS` → UI「未缓存」 |
| DYNAMIC | 请求时不 eligible | 策略 skip 多为 `BYPASS` 或空 → UI「未缓存」 |

---

## 3. 产品语义

### 3.1 双层开关

* **性能策略总开关**：节点绑定的性能策略中配置 `cache_enabled`；关闭则该节点不启用本地缓存存储。  
* **路由级缓存开关**：网站规则（Proxy Route）配置 `cache_enabled`，控制该站点是否执行缓存链路。

两者均开启时才进入缓存逻辑。

### 3.2 策略枚举

| `cache_policy` | 含义 | 新建默认 | 旧值兼容 |
| --- | --- | --- | --- |
| **`static`** | 仅 URI 匹配**标准静态扩展名**才 eligible | **是** | — |
| **`all`** | 过方法旁路后，不限制路径/扩展名（高级，风险类似 CF Cache Everything） | 否 | 存量 `url` → `all` |
| **`suffix`** | 自定义扩展名列表（`cache_rules`） | 否 | 保持 |
| **`path_prefix`** | 自定义路径前缀 | 否 | 保持 |
| **`path_exact`** | 自定义精确路径 | 否 | 保持 |

API/UI 统一暴露上表枚举；历史值 `url` 在读写时自动规范化为 `all`。

### 3.3 标准静态扩展名（内置）

对齐 CF 默认「不缓存 HTML/JSON」；保留现代前端常用增强项：

```text
css js mjs map
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

* **不含** `html` / `htm` / **`json`**（对齐 CF 默认不缓存 JSON）。  
* **含** `map` / `mjs` / `wasm`（提高 sourcemap / ES module / WASM 命中）。  
* 匹配大小写不敏感扩展名。

### 3.4 请求侧旁路

仅保留：

1. `$request_method != GET`（含 HEAD）

**删除过往过严条件（恢复正常命中率）：**

* 会话类 Cookie 过滤  
* 请求头 `Authorization` 一律旁路  
* 客户端请求头 `Cache-Control: no-cache|no-store|private` 一律旁路  

**安全边界防护：**

| 威胁 | 闸门 |
| --- | --- |
| 误缓存 HTML/API | 默认 `static` 扩展名（不含 html/json） |
| 个性化内容 | 源站 `private` / `no-store`（网关严格遵守） |
| 响应写会话 | **`Set-Cookie` → 不入库**（§3.6） |
| `all` 过宽 | UI/文档警告：需源站正确声明 Cache-Control |
| 带 Bearer 的 API | 依赖策略（勿对 API 用 `all`）+ 源站头 |

### 3.5 默认 Edge TTL（无源站 freshness 时）

对齐 CF 无 `Cache-Control`/`Expires` 时的状态码默认 TTL：

| 状态码 | TTL |
| --- | --- |
| 200, 206, 301 | 120m (7200s) |
| 302, 303 | 20m (1200s) |
| 404, 410 | 3m (180s) |

* 源站提供合法 `Cache-Control` / `Expires` 时，以源站 freshness 为准。  
* 默认不执行强制忽略源站头的 Edge TTL 覆盖。

### 3.6 响应侧：Set-Cookie 不入库

对齐 CF OCC 默认：eligible 请求若源站返回 **`Set-Cookie`**，**不写入** 缓存存储。

响应侧只影响**写入**，与 CF「eligible 但响应不可缓存」一致。

### 3.7 与源站头的关系

* **是否 eligible**：策略 + 请求方法。  
* **是否入库 / 存多久**：源站 `Cache-Control` / `Expires` + 默认 Edge TTL + Set-Cookie 闸门 + 性能策略存储限制。  

---

## 4. 运行时与数据流

```text
节点性能策略 cache_enabled?
    │ no → 节点不启用缓存存储
    ▼ yes
路由配置 cache_enabled?
    │ no → 该站点不执行缓存
    ▼ yes
请求处理阶段 (GET / HEAD)
    → 策略匹配（static / all / suffix / path_prefix / path_exact）
    → 不匹配策略 → SKIP (BYPASS)
    → 匹配策略 → 生成 Cache Key，查询 TinyUFO 内存/磁盘热层
    → 命中且有效 → 直接返回缓存响应 (HIT)
回源响应阶段
    → 检查源站 Cache-Control / Expires
    → 包含 Set-Cookie 或 private / no-store → 响应客户端但不写入缓存
    → 无源站缓存头 → 应用状态码默认 Edge TTL
    → 写入缓存存储并向客户端返回
    → 上报 access log cache_status
```

### 4.1 涉及代码面

| 区域 | 路径 |
| --- | --- |
| 边缘缓存运行时 | `edge/pingora/src/proxy/cache.rs`（OpenFlare Edge 缓存键、TTL、判定与 TinyUFO 存储） |
| 性能策略与存储配置 | `internal/openflare/performance/`、`edge/pingora/src/runtime/performance.rs` |
| 路由快照编译 | `internal/openflare/edgeconfig/`、`internal/apps/openflare/config_version/snapshot.go` |
| 校验 | `internal/apps/openflare/proxy_route/helpers.go` |
| 模型/默认 | 创建路由默认 `cache_policy=static`；读写时 `url`→`all` |
| UI | `frontend/app/(main)/proxy-routes/detail/components/cache-section.tsx` |

---

## 5. 兼容与迁移

| 数据 | 处理 |
| --- | --- |
| DB 中 `cache_policy=''` 或 `url`（且已启用缓存） | 读 / 快照 / 渲染 → **`all`** |
| API 写入 enabled 且 policy 为空 | 规范为 **`all`**；UI 新建开启时**显式提交** `static` |
| 新建路由 | 开启缓存时默认 **`static`** |
| 旁路行为变更 | 带 Cookie/Auth 的请求从一律「未缓存」恢复为符合条件的静态资源可命中；控制端自动同步快照后生效 |
| 默认扩展名 | 自表中 **移除 `json`**；已依赖缓存 `*.json` 的站点可改 `suffix` 自定义或 `all` |

---

## 6. UI 文案要点（缓存 Tab）

* 开启缓存后默认：**标准静态资源**（扩展名匹配，**不含 HTML/JSON**；含 map/mjs 等）。  
* 选项：标准静态 / 所有可缓存 GET（高级）/ 自定义后缀 / 路径前缀 / 精确路径。  
* 说明对齐 CF：  
  * 登录 Cookie **不会**单独跳过缓存；  
  * 源站 `private` / `no-store` / 响应 **`Set-Cookie`** 不会写入边缘缓存；  
  * 无源站缓存头时使用默认 Edge TTL。  
* **高级 `all`**：提示「类似 Cache Everything，个性化页面必须由源站声明 private/no-store」。  
* 节点绑定的性能策略中必须开启缓存总开关。

---

## 7. 决策矩阵

| 场景 | CF | OpenFlare（本设计） |
| --- | --- | --- |
| GET 静态 + session Cookie + 源站 public max-age | HIT | HIT |
| GET HTML + static 策略 | DYNAMIC | 策略 skip → 未缓存 |
| GET + all + 源站 private | 不入库 | 不入库 |
| GET 静态 + 响应 Set-Cookie | BYPASS（OCC） | 不入库 |
| GET + Authorization + 静态 public | 条件缓存 | 可缓存（简化；依赖源站正确标记 public） |
| GET + 无 CC 的 200 静态 | 默认 120m | 默认 TTL 120m |
| DevTools Disable cache（请求 no-cache） | 边缘默认可仍 HIT | 边缘默认可仍 HIT |
| POST | 不缓存 | 非 GET skip |

---

## 8. 决策记录

| 决策 | 选择 | 原因 |
| --- | --- | --- |
| 请求 Cookie 旁路 | **删除** | 对齐 CF；恢复登录用户静态资源命中率 |
| 请求 Authorization / Cache-Control 旁路 | **删除** | 对齐 CF 请求 eligible 模型；响应闸门兜底 |
| Set-Cookie | **禁止入库** | 对齐 CF OCC「响应 Set-Cookie 不入库」 |
| 默认 Edge TTL | **按状态码默认 TTL** | 对齐 CF 无头时默认 TTL，避免资源无法入库 |
| 默认表去掉 json | **是** | 对齐 CF 默认不缓存 JSON |
| 保留 map/mjs/wasm | **是** | 提升前端构建产物缓存命中 |
| 默认可缓存范围 | 开启缓存默认 `static` | 对标 CF，降低 HTML/API 误缓存风险 |
| 旧 `url` | 映射 `all` | 存量兼容 |
