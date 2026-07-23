# 边缘缓存策略设计

你会学到：OpenFlare 边缘 `proxy_cache` 如何在「该缓存」与「不该缓存」之间对齐 Cloudflare 默认闭环：请求 eligible（扩展名/策略）× 响应可共享缓存（源站 `Cache-Control` / `Expires` / `Set-Cookie`），以及与过往过严请求旁路的差异。

本设计是 [系统架构](./architecture.md) 中「基础缓存」的产品化专章；访问日志中的缓存结果见 [观测数据模型 §3.5.1](./observability-data-model.md)。

---

## 1. 目标与非目标

### 1.1 目标

* **开箱接近 CF 默认**：路由开启缓存后，**默认只缓存静态扩展名**，不默认缓存 HTML；**不因请求会话 Cookie / Authorization / 客户端 Cache-Control 一律 BYPASS**。
* **该缓存的能命中**：带登录 Cookie 的用户访问 `/_app/**/*.js` 等静态资源可出现 `MISS` → `HIT`。
* **不该缓存的仍挡住**：策略不 eligible（等价 CF `DYNAMIC`）；源站 `private` / `no-store`；响应带 **`Set-Cookie` 不入库**（对齐 CF OCC 默认）；`all` 为高级选项并文档警示。
* **无源站 freshness 时有默认 Edge TTL**：对齐 CF 按状态码的默认 TTL（见 §3.5）。
* **可观测一致**：继续依赖 `$upstream_cache_status` → `cache_status` 明细三态。
* **兼容存量**：旧路由 `cache_policy=url` 映射为 `all`；策略枚举与迁移规则保持 [§5](#5-兼容与迁移)。

### 1.2 非目标（后续迭代）

* Cache Rules 表达式引擎  
* 忽略源站 `Cache-Control` 的强制 Edge TTL（CF Cache Rules「Ignore cache-control」）  
* Purge（按 URL/前缀/全站）  
* 浏览器 TTL 改写、客户端 `CF-Cache-Status` 响应头  
* 完整 RFC 条件：`Authorization` 仅当响应含 `public`/`s-maxage`/`must-revalidate` 才缓存（需 Lua；本期删除请求侧一律旁路，依赖策略 + 源站头）  
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
| 请求 `Authorization` | 仅当响应另有 `public` / `s-maxage` / `must-revalidate` 才可缓存（完整条件本期用 Nginx 简化，见 §3.4） |

### 2.3 状态语义（对照观测）

| CF | 含义 | OpenFlare `cache_status` |
| --- | --- | --- |
| HIT / STALE / UPDATING / REVALIDATED | 命中类 | 同名或等价 |
| MISS / EXPIRED | 回源取内容 | 同名 |
| BYPASS | 请求时 eligible，响应不可缓存 | `BYPASS` → UI「未缓存」 |
| DYNAMIC | 请求时不 eligible | 策略 skip 多为 `BYPASS` 或空 → UI「未缓存」 |

---

## 3. 产品语义

### 3.1 双层开关（不变）

* **全局** `openresty_cache_enabled`：生成 `proxy_cache_path` 等；关闭则路由级缓存指令不生效。  
* **路由** `cache_enabled`：是否在该站点 `location` 启用 `proxy_cache`。

两者均开启时才进入缓存逻辑。

### 3.2 策略枚举

| `cache_policy` | 含义 | 新建默认 | 旧值兼容 |
| --- | --- | --- | --- |
| **`static`** | 仅 URI 匹配**标准静态扩展名**才 eligible | **是** | — |
| **`all`** | 过方法旁路后，不限制路径/扩展名（高级，风险类似 CF Cache Everything） | 否 | 存量 `url` → `all` |
| **`suffix`** | 自定义扩展名列表（`cache_rules`） | 否 | 保持 |
| **`path_prefix`** | 自定义路径前缀 | 否 | 保持 |
| **`path_exact`** | 自定义精确路径 | 否 | 保持 |

渲染层：历史值 `url` 按 `all` 处理；API/UI 只暴露上表枚举。

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
* **含** `map` / `mjs` / `wasm`（有意增强，提高 sourcemap / ES module / WASM 命中）。  
* 匹配：`$uri` 扩展名，大小写不敏感：  
  `if ($uri !~* \.(?:css|js|…)$) { set $openflare_skip_cache 1; }`

### 3.4 请求侧旁路（对齐 CF 后）

仅保留：

1. `$request_method != GET`（含 HEAD，与现网一致；不做 CF 的 HEAD→GET）

**删除（过往过严，导致缓存率过低）：**

* 会话类 Cookie 正则  
* `$http_authorization != ""`  
* 请求 `$http_cache_control` 匹配 `no-cache|no-store|private`

**安全如何仍成立：**

| 威胁 | 闸门 |
| --- | --- |
| 误缓存 HTML/API | 默认 `static` 扩展名（不含 html/json） |
| 个性化内容 | 源站 `private` / `no-store`（Nginx 尊重） |
| 响应写会话 | **`Set-Cookie` → 不入库**（§3.6） |
| `all` 过宽 | UI/文档警告：需源站正确 Cache-Control |
| 带 Bearer 的 API | 依赖策略（勿对 API 用 `all`）+ 源站头；完整 Auth 条件缓存为后续 |

### 3.5 默认 Edge TTL（无源站 freshness 时）

对齐 CF 无 `Cache-Control`/`Expires` 时的状态码默认 TTL，在启用缓存的 location 输出：

| 状态码 | TTL |
| --- | --- |
| 200, 206, 301 | 120m |
| 302, 303 | 20m |
| 404, 410 | 3m |

```nginx
proxy_cache_valid 200 206 301 120m;
proxy_cache_valid 302 303 20m;
proxy_cache_valid 404 410 3m;
```

* 源站提供合法 `Cache-Control` / `Expires` 时，仍以源站 freshness 为准（不 `proxy_ignore_headers`）。  
* **不做**强制忽略源站头的 Edge TTL 覆盖。

### 3.6 响应侧：Set-Cookie 不入库

对齐 CF OCC 默认：eligible 请求若源站返回 **`Set-Cookie`**，**不写入** `proxy_cache`（可读路径仍可能 MISS/BYPASS 语义）。

```nginx
proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie;
```

（`proxy_no_cache` 多参数：任一非空且非 `"0"` 则不写入。）

`proxy_cache_bypass` 仍仅绑定 `$openflare_skip_cache`（请求侧 skip）；响应侧只影响**写入**，与 CF「eligible 但响应不可缓存」一致。

### 3.7 与源站头的关系

* **是否 eligible**：策略 + 方法旁路。  
* **是否入库 / 存多久**：源站 `Cache-Control` / `Expires` + 默认 `proxy_cache_valid` + Set-Cookie 闸门 + 全局 `inactive`。  

---

## 4. 渲染与数据流

```text
全局 cache_enabled?
    │ no → 不生成 proxy_cache_*
    ▼ yes
路由 cache_enabled?
    │ no → location 无 proxy_cache
    ▼ yes
set $openflare_skip_cache 0
    → 非 GET → 置 1
    → 策略 if（static/all/suffix/…）→ 可置 1
proxy_cache openflare_cache
proxy_cache_methods GET
proxy_cache_bypass $openflare_skip_cache
proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie
proxy_cache_valid …
    →
access.log cache_status=$upstream_cache_status
```

### 4.1 策略 → Nginx 条件

| 策略 | 额外条件 |
| --- | --- |
| `static` | `$uri` 不匹配内置扩展名表 → skip |
| `all` | 无额外路径条件 |
| `suffix` | 不匹配 `cache_rules` 扩展名 → skip |
| `path_prefix` / `path_exact` | 同现实现 |

### 4.2 涉及代码面

| 区域 | 路径 |
| --- | --- |
| 渲染 | `pkg/render/openresty/render.go`（旁路、Set-Cookie、`proxy_cache_valid`、扩展名常量） |
| 校验 | `internal/apps/openflare/proxy_route/helpers.go` |
| 模型/默认 | 创建路由默认 `cache_policy=static`；读写时 `url`→`all` |
| 快照 | `config_version` 快照规范化 |
| UI | `proxy-routes/detail/components/cache-section.tsx` |
| 测试 | `pkg/render/openresty/render_test.go` 等 |

---

## 5. 兼容与迁移

| 数据 | 处理 |
| --- | --- |
| DB 中 `cache_policy=''` 或 `url`（且已启用缓存） | 读 / 快照 / 渲染 → **`all`** |
| API 写入 enabled 且 policy 为空 | 规范为 **`all`**；UI 新建开启时**显式提交** `static` |
| 新建路由 | 开启缓存时默认 **`static`** |
| 旁路行为变更 | **破坏性相对旧实现**：带 Cookie/Auth 的流量从「未缓存」变为可 HIT；需 **重新发布节点配置** 后生效 |
| 默认扩展名 | 自表中 **移除 `json`**；已依赖缓存 `*.json` 的站点可改 `suffix` 自定义或 `all` |

**发布说明：** 说明本次对齐 CF 默认模型；命中率预期上升；`all` 与错误源站头风险需运维自查。

---

## 6. UI 文案要点（缓存 Tab）

* 开启缓存后默认：**标准静态资源**（摘要扩展名，**不含 HTML/JSON**；含 map/mjs 等）。  
* 选项：标准静态 / 所有可缓存 GET（高级）/ 自定义后缀 / 路径前缀 / 精确路径。  
* 说明对齐 CF：  
  * 登录 Cookie **不会**单独跳过缓存；  
  * 源站 `private` / `no-store` / 响应 **`Set-Cookie`** 不会写入边缘缓存；  
  * 无源站缓存头时使用默认 Edge TTL。  
* **高级 `all`**：警告「类似 Cache Everything，个性化页面必须由源站声明 private/no-store」。  
* 全局 Performance 缓存总开关须开启。

---

## 7. 验证要点

* 渲染：无 Cookie/Auth/请求 Cache-Control 旁路；含 `proxy_cache_valid` 三行；`proxy_no_cache` 含 `$upstream_http_set_cookie`。  
* 单测：内置表含 `css`/`js`/`map`/`mjs`，**不含** `html`/`json`。  
* 手动：  
  * 带 session Cookie 请求 `/a.js` → 第二次 `HIT`；  
  * `/index.html` + `static` → 未缓存；  
  * 源站对 eligible 路径返回 `Set-Cookie` → 不入库（持续 MISS/不 HIT）；  
  * 源站 `Cache-Control: private` → 不入库。  
* 观测：access log 三态与原始 `cache_status` 一致。  
* 生效：配置版本发布并节点应用后验证。

---

## 8. 决策矩阵（防漏判）

| 场景 | CF | OpenFlare（本设计） |
| --- | --- | --- |
| GET 静态 + session Cookie + 源站 public max-age | HIT | HIT |
| GET HTML + static 策略 | DYNAMIC | 策略 skip → 未缓存 |
| GET + all + 源站 private | 不入库 | 不入库 |
| GET 静态 + 响应 Set-Cookie | BYPASS（OCC） | 不入库 |
| GET + Authorization + 静态 public | 条件缓存 | 可缓存（简化；依赖源站勿对敏感 API 乱标 public） |
| GET + 无 CC 的 200 静态 | 默认 120m | `proxy_cache_valid` 120m |
| DevTools Disable cache（请求 no-cache） | 边缘默认可仍 HIT | 边缘默认可仍 HIT |
| POST | 不缓存 | 非 GET skip |

---

## 9. 后续路线图

1. Auth 完整 RFC/CF 条件缓存（Lua）  
2. 强制 Edge TTL / `proxy_ignore_headers`（Cache Rules 级）  
3. Purge API  
4. Cache Rules（有序规则 + 动作）  
5. 全局默认可缓存扩展名可配置；可选对齐 CF 更长扩展名表  
6. HEAD→GET  

---

## 10. 决策记录

| 决策 | 选择 | 原因 |
| --- | --- | --- |
| 请求 Cookie 旁路 | **删除** | 对齐 CF；恢复登录用户静态命中率 |
| 请求 Authorization / Cache-Control 旁路 | **删除** | 对齐 CF 请求 eligible 模型；响应闸门兜底 |
| Set-Cookie | **proxy_no_cache 绑定** | 对齐 CF OCC「响应 Set-Cookie 不入库」 |
| 默认 Edge TTL | **按状态码 proxy_cache_valid** | 对齐 CF 无头时默认 TTL，避免「永不入库」 |
| 默认表去掉 json | **是** | 对齐 CF 默认不缓存 JSON |
| 保留 map/mjs/wasm | **是** | 现代前端有用命中，有意增强 |
| 默认可缓存范围 | 开启缓存默认 `static` | 对标 CF，降低 HTML/API 误缓存 |
| 旧 `url` | 映射 `all` | 存量行为不收窄 |
| 完整 Auth 条件 / Purge / Rules | 后续 | 先闭合默认闭环再扩展 |
