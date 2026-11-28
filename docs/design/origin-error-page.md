# 源站错误页设计

当源站或边缘网关产生指定 HTTP 错误状态码时，OpenFlare Edge 使用全局配置的自定义或内置默认 HTML 页面替代原始错误响应，同时严格保持真实的 HTTP 响应状态码并在正文中注入当前状态码与请求域名。

---

## 1. 行为边界与产品语义

* **全局统一配置**：错误页作为全局运维配置存在于控制台「网站管理 → 响应页面」（`/responses`、`/responses/error-page/edit`、`/responses/error-page/preview`），全站反向代理路由共用；**不适用于** Pages 静态托管路由、WAF/PoW/限流自带拦截页面（除非主动将对应状态码纳入列表）以及命中陈旧缓存（`stale`）成功响应的场景。
* **状态码严格透传**：网关仅替换响应正文并设置 `Content-Type: text/html; charset=utf-8`，HTTP 响应码保持原始值（如 `500`、`502`、`504`），绝不篡改为 `200`。
* **GET-only 模式**：开启 `origin_error_page_get_only` 时，仅替换 `GET` 请求的错误响应；`POST`、API 回调等非 `GET` 请求保持透传原始响应。
* **模板与占位符**：内置极简风格 HTML 模板（`internal/openflare/edgeconfig/origin_error_page.go`）直接内嵌品牌 SVG 标识，无需外部资源请求；运行时支持 `{{status}}` 与 `{{host}}` 内存替换。由于属于管理员全局运维配置，存储时不做 XSS 消毒（界面提示勿嵌入不可信第三方脚本）。

---

## 2. 配置模型与编译流

配置项作为标准 Option 持久化于 `w_system_configs`（复用 `GET /api/v1/d/option` 与 `POST /api/v1/d/option/update-batch`），并在编译站点快照时打包进 `sites.v1` 下发至边缘节点：

| 配置键 | 类型 | 默认值 | 校验与约束 |
| :--- | :--- | :--- | :--- |
| `origin_error_page_enabled` | bool 字符串 | `true` | 总开关；关闭后所有请求透传原始响应 |
| `origin_error_page_status_codes` | JSON 字符串数组 | `["500-599"]` | 支持单码（如 `522`）或闭区间（如 `500-599`），展开后须位于 `400–599` 且启用时非空 |
| `origin_error_page_html` | 文本 | `""` | 空字符串表示使用内置默认模板；字节上限 `256 KiB` |
| `origin_error_page_get_only` | bool 字符串 | `false` | 控制是否仅拦截替换 `GET` 请求错误 |

标签展开与校验逻辑由共享纯函数实现（`internal/openflare/edgeconfig/origin_error_page.go`），确保 API 校验与快照编译语义完全一致；`openflare-edge` 在原子切换 `sites.v1` 后立即在内存热生效。
