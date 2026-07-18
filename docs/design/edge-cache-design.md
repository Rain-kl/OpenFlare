# 边缘缓存策略设计（对标 Cloudflare 默认可缓存范围）

你会学到：OpenFlare 边缘 `proxy_cache` 的产品边界、默认可缓存范围如何对齐 Cloudflare「静态资源默认可缓存」、策略枚举与渲染规则、兼容迁移，以及本阶段明确不做的能力。

本设计是 [系统架构](./architecture.md) 中「基础缓存」的产品化专章；访问日志中的缓存结果见 [观测数据模型 §3.5.1](./observability-data-model.md)。

---

## 1. 目标与非目标

### 1.1 目标（第一期）

* **开箱接近 CF 默认**：路由开启缓存后，**默认只缓存静态扩展名**，不默认缓存 HTML/无扩展名动态路径。
* **行为可解释**：与现有安全旁路（非 GET、Authorization、会话 Cookie、请求 `Cache-Control`）叠加，不削弱安全。
* **可观测一致**：继续依赖 `$upstream_cache_status` → `cache_status` 明细三态。
* **兼容存量**：旧路由 `cache_policy=url`（近似「过旁路即可缓存」）迁移为显式策略 `all`，行为不变。

### 1.2 非目标（后续迭代）

* Cache Rules 表达式引擎  
* Edge TTL / `proxy_cache_valid` / 忽略源站 `Cache-Control`  
* 可配置 Cookie 旁路列表、Query 忽略列表  
* Purge（按 URL/前缀/全站）  
* 浏览器 TTL 改写、客户端 `CF-Cache-Status` 响应头  
* 命中率看板  

---

## 2. 现状摘要

| 层 | 现状 |
| --- | --- |
| 全局 | `proxy_cache_path` / key / lock / stale（Performance 部分字段） |
| 路由 | `cache_enabled` + `cache_policy`：`url` \| `suffix` \| `path_prefix` \| `path_exact` |
| 旁路 | 渲染器硬编码：非 GET、Authorization、会话 Cookie、请求 Cache-Control |
| TTL | **无** `proxy_cache_valid`；存多久主要看源站头 + `inactive` |
| 观测 | 已上报 `cache_status`，UI 三态：命中 / 回源 / 未缓存 |

问题：默认策略 `url` 对「过旁路的 GET」范围过宽，与 CF「默认主要缓存静态扩展名、默认不缓存 HTML」不一致。

---

## 3. 产品语义

### 3.1 双层开关（不变）

* **全局** `openresty_cache_enabled`：生成 `proxy_cache_path` 等；关闭则路由级缓存指令不生效。  
* **路由** `cache_enabled`：是否在该站点 `location` 启用 `proxy_cache`。

两者均开启时才进入缓存逻辑。

### 3.2 策略枚举（第一期）

| `cache_policy` | 含义 | 新建默认 | 旧值兼容 |
| --- | --- | --- | --- |
| **`static`** | 仅 URI 匹配**标准静态扩展名**（内置表）才允许缓存 | **是** | — |
| **`all`** | 过安全旁路后，不限制路径/扩展名（等同今日 `url`） | 否 | 存量 `url` → `all` |
| **`suffix`** | 自定义扩展名列表（`cache_rules`） | 否 | 保持 |
| **`path_prefix`** | 自定义路径前缀 | 否 | 保持 |
| **`path_exact`** | 自定义精确路径 | 否 | 保持 |

> 渲染层：读到历史值 `url` 时按 `all` 处理，避免未迁移数据行为突变；API 校验与 UI 只暴露上表枚举（写入时可将 `url` 规范为 `all`）。

### 3.3 标准静态扩展名（内置，V1 硬编码）

对齐 Cloudflare 常见「默认可缓存静态」集合，**默认不包含** `html` / `htm`：

```text
css js mjs map json
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

* 匹配对象：`$uri` 的扩展名（大小写不敏感），实现上与现有 `suffix` 策略相同：  
  `if ($uri !~* \.(?:css|js|…)$) { set $openflare_skip_cache 1; }`  
* **V1.1（可选）**：全局配置项覆盖该列表；第一期不强制。

### 3.4 安全旁路（保持硬编码）

在策略匹配之前/之外，仍设置 `$openflare_skip_cache=1`：

1. `$request_method != GET`（含 HEAD，与现网一致）  
2. `$http_authorization != ""`  
3. 会话类 Cookie 正则（现网列表）  
4. 请求 `$http_cache_control` 匹配 `no-cache|no-store|private`  

`proxy_cache_bypass` / `proxy_no_cache` 均绑定 `$openflare_skip_cache`。

### 3.5 与源站头的关系（本阶段不改）

* 仍不输出 `proxy_cache_valid`。  
* 对象**是否进入缓存流程**由策略 + 旁路决定；**存多久**继续依赖源站 `Cache-Control` / `Expires` 等及全局 `inactive`。  
* Edge TTL / 强制忽略源站头 → 后续专项。

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
    → 安全旁路 if → 置 1
    → 策略 if（static/all/suffix/…）→ 可置 1
proxy_cache openflare_cache
proxy_cache_methods GET
proxy_cache_bypass / proxy_no_cache $openflare_skip_cache
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

### 4.2 涉及代码面（实现时）

| 区域 | 路径 |
| --- | --- |
| 渲染 | `pkg/render/openresty/render.go`（策略分支 + 内置扩展名常量） |
| 校验 | `internal/apps/openflare/proxy_route/helpers.go` |
| 模型/默认 | 创建路由默认 `cache_policy=static`；读写时 `url`→`all` |
| 快照 | `config_version/snapshot.go` |
| UI | `proxy-routes/detail/components/cache-section.tsx` |
| 测试 | `pkg/render/openresty/render_test.go`、proxy_route helpers 测试 |

---

## 5. 兼容与迁移

| 数据 | 处理 |
| --- | --- |
| DB 中 `cache_policy=''` 或 `url`（且已启用缓存） | 读取 / 快照 / 渲染均规范为 **`all`**，保证存量「宽缓存」不变 |
| API 写入时 `enabled` 且 policy 为空 | 规范为 **`all`**（兼容旧客户端）；UI 新建开启时**显式提交** `static` |
| 新建路由 | 默认 `cache_enabled=false`；表单开启缓存时默认策略 **`static`** |
| 已开启且 `url` 的站点 | 显示与发布为 `all`，**缓存范围不变** |
| 期望「只缓存静态」的旧站点 | 用户在 UI 改为 `static` 或自定义 `suffix` |

**发布说明建议：** 说明默认策略变更仅影响**新配置**；存量 `url` 视为 `all`。

---

## 6. UI 文案要点（缓存 Tab）

* 开启缓存后默认：**标准静态资源**（列出扩展名摘要，并写明不含 HTML）。  
* 选项：**标准静态资源** / **所有可缓存 GET（高级）** / 自定义后缀 / 路径前缀 / 精确路径。  
* 固定说明：非 GET、带 Authorization、常见登录 Cookie、请求禁止缓存头时跳过缓存。  
* 提示：全局 Performance 中缓存总开关须开启，否则站点开关无效。

---

## 7. 验证要点

* 渲染：`static` 生成扩展名 `if`；`all`/`url` 无路径限制；旁路四条仍在。  
* 单测：内置表含 `css`/`js`/`woff2`，不含 `html`。  
* 手动：开启 `static` 后请求 `/a.css` 可出现 HIT/MISS；`/index.html` 或 `/api` 多为未缓存/BYPASS。  
* 观测：access log `cache_status` 与列表三态一致。

---

## 8. 后续路线图（非本设计交付）

1. **Edge TTL / 尊重源站开关**（`proxy_cache_valid`、`proxy_ignore_headers`）  
2. **可配置旁路**（Cookie/Query）  
3. **Purge API**  
4. **Cache Rules**（有序规则 + 动作）  
5. **全局默认可缓存扩展名配置**  

---

## 9. 决策记录

| 决策 | 选择 | 原因 |
| --- | --- | --- |
| 默认可缓存范围 | 开启缓存默认 `static` 扩展名表 | 对标 CF 开箱行为，降低 HTML/API 被误缓存 |
| 旧 `url` | 映射为 `all` | 避免存量站点行为变化 |
| HTML | 默认不在白名单 | 对齐 CF 默认不缓存 HTML |
| 第一期不做 Edge TTL/Purge | 明确 Out of Scope | 先收敛「谁可以进缓存」再优化「存多久/怎么清」 |
