# 源站错误页设计

你会学到：源站或网关返回指定错误状态码时，OpenFlare 如何用全局可配置页面替代透传响应；配置如何进入不可变配置版本，以及边缘 OpenResty 如何保持真实 HTTP 状态码并在页面中展示该状态码。

本设计是 [系统架构](./architecture.md) 中反代流量路径的产品化补充；配置发布模型见 [Agent 与发布模型](./agent-design.md)。

---

## 1. 目标与非目标

### 1.1 目标

* **可拦截**：在用户配置的状态码集合上，用统一 HTML 替换原先透传的源站/Nginx 默认错误响应。
* **可关闭**：全局开关关闭后行为与现状一致（透传 / Nginx 默认页）。
* **默认可视**：默认启用，默认状态码标签 `500-599`，默认 Cloudflare 风格错误页（无 CF 商标）。
* **可自定义**：管理员可在线编辑完整 HTML；空 HTML 表示使用内置默认模板。
* **状态码透传**：HTTP 响应 `status` 保持原错误码（如 502、522）；页面正文通过 `{{status}}` 展示同一数值。
* **全局统一**：侧栏「网站管理 → 错误页」单一配置，全站反代路由共用。
* **与发布一致**：配置经 Option 持久化，进入配置版本快照后随发布/回滚下发。

### 1.2 非目标

* 按反代路由 / Zone 覆盖错误页  
* 通过上传文件托管错误页（仅在线 HTML）  
* 修改 WAF / PoW / 限流自有响应页（除非用户把对应状态码加入列表）  
* Pages 静态路由错误页  
* 多语言错误页、品牌资源 CDN  

---

## 2. 产品行为

### 2.1 何时替换

| 条件 | 行为 |
| --- | --- |
| 开关开启，且响应状态码落在展开后的集合内 | 返回自定义/默认 HTML，**status 不变** |
| 开关关闭 | 不生成 `error_page` 相关指令，透传 |
| 状态码不在集合内 | 不替换 |
| Pages 上游路由 | 不应用本功能 |
| 源站成功返回 2xx/3xx/4xx（未配置时） | 不替换 |

实现上对反代 `location` 启用 `proxy_intercept_errors on`，因此**源站返回的**匹配 5xx 等也会被拦截，而不仅是网关本地生成的 502。

### 2.2 状态码标签语法

Tags Input 每条标签：

| 形式 | 示例 | 含义 |
| --- | --- | --- |
| 单码 | `522` | 仅该码 |
| 闭区间 | `500-599` | 含端点展开 |

* 合法范围：单码与区间两端均在 **400–599**；`lo ≤ hi`。  
* 默认标签列表：`["500-599"]`。  
* 持久化存**原始标签**（JSON 数组字符串）；渲染时展开、去重、排序。  
* 启用时展开结果为空 → 保存拒绝。  
* 非法标签 → 保存拒绝并返回可读错误。

### 2.3 页面占位符

| 占位符 | 含义 |
| --- | --- |
| `{{status}}` | 当前响应状态码（与 HTTP status 一致） |
| `{{host}}` | 请求 Host |

自定义 HTML 与默认模板均支持上述占位符；运行时在边缘替换。未使用的占位符可不出现在模板中。

### 2.4 默认页

内置 Cloudflare 风格：浅色居中、大号状态码、简短英文/中性说明、小字 Host。不使用 Cloudflare 商标或 Ray ID 伪造。前后端共用同一默认 HTML 常量（或同源字符串），前端「加载默认模板」直接填入编辑器。

---

## 3. 配置模型

### 3.1 Option keys（`w_system_configs` / OpenFlare Option API）

| Key | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | bool 字符串 | `true` | 总开关 |
| `origin_error_page_status_codes` | JSON 字符串数组 | `["500-599"]` | 原始标签 |
| `origin_error_page_html` | 文本 | `""` | 空 = 内置默认；最大 **256 KiB** |

API 复用：

* `GET /api/v1/d/option`  
* `POST /api/v1/d/option/update-batch`  

不新增独立资源路由。goose 迁移写入 seed；常量定义于 `internal/model` 配置 key 区。

### 3.2 校验（update-batch）

1. `enabled`：可解析为 bool。  
2. `status_codes`：合法 JSON 数组；每项 `^\d{3}$` 或 `^\d{3}-\d{3}$`；展开后均在 400–599；启用时非空。  
3. `html`：长度 ≤ 256 KiB（按字节）；允许空。  
4. 解析/展开逻辑为**纯函数**，供 API 与 `pkg/render/openresty` 共用，避免前后端/渲染语义分叉。

不对 HTML 做 XSS 消毒：属管理员全局运维配置，与边缘公开展示一致；文档提示勿嵌入不可信第三方脚本。

### 3.3 配置版本快照

`ConfigSnapshot` 增加字段：

```text
OriginErrorPageEnabled     bool
OriginErrorPageStatusCodes []string  // 原始标签
OriginErrorPageHTML        string    // 空则渲染器用内置默认
```

构建快照时从 Option 读取；Agent 只消费快照，不直读控制面 DB。

---

## 4. 边缘渲染

### 4.1 启用时生成内容

1. **SupportFile**：错误页模板（如 `error_pages/origin_error.html.tmpl`），内容为自定义 HTML 或内置默认，保留 `{{status}}` / `{{host}}`。  
2. **每个反代 proxy server**（含 HTTP/HTTPS 反代；不含 Pages）：

```nginx
proxy_intercept_errors on;
error_page <expanded codes...> = /__openflare_origin_error;

location = /__openflare_origin_error {
    internal;
    default_type text/html;
    charset utf-8;
    # 保持 ngx.status 为原错误码
    # 读取模板，替换 {{status}} / {{host}} 后输出 body
}
```

### 4.2 运行时替换

采用 **internal location 内轻量 Lua（或现有 resty 能力）** 读模板并 `string.gsub` 替换占位符，**不**把 status 固化进静态文件（请求间状态码不同）。

禁止将错误页统一改为 HTTP 200。

### 4.3 关闭时

不输出 `proxy_intercept_errors`、`error_page`、内部 location 与对应 SupportFile（或文件可写但不被引用）。

### 4.4 与缓存 / stale

若全局 `proxy_cache_use_stale` 在部分错误码上返回过期缓存，**成功返回 stale 内容时不会进入 error_page**。仅当实际上对客户端产生配置列表内错误状态时才展示错误页。行为依赖现有缓存指令，本功能不改 stale 策略。

---

## 5. 前端

### 5.1 入口

* 侧栏「网站管理」新增：**错误页** → `/error-pages`  
* 更新 `openflareWebsiteNavGroup`、`openflareWebsiteSubNav`（若使用）、全局搜索关键词  

### 5.2 页面结构

* 页头说明：保存后需到「版本发布」发布才生效。  
* **开关 + Tags Input**（shadcn-extension Tags Input：`@/components/ui/tags-input`）：状态码标签。  
* **HTML 编辑区** +「加载默认模板」「恢复默认（清空）」+ 占位符说明。  
* **客户端预览**：用示例 `status=502`、`host=example.com` 替换后 sandbox/iframe 预览。  
* 保存：`OptionService.updateBatch`；权限与性能调优页一致（管理员）。  

### 5.3 组件依赖

若仓库尚无 Tags Input，按项目 shadcn 流程添加；样式与现有 UI 一致。

---

## 6. 数据流

```text
管理员 /error-pages
    → Option update-batch（校验标签与 HTML）
    → w_system_configs

发布配置版本
    → 快照写入 OriginErrorPage*
    → 渲染 OpenResty conf + SupportFile
    → Agent 拉取并 reload

访客请求反代域名
    → 源站/网关产生匹配状态码
    → error_page → internal location
    → 替换占位符，status 保持原码，返回 HTML
```

---

## 7. 测试与验收

### 7.1 自动化

* 状态码解析：单码、区间、去重、越界、反序、默认 `500-599`  
* 渲染：enabled/disabled conf 片段；空 HTML 用默认；自定义进 SupportFile  
* Option 校验：非法标签 / 超大 HTML → 4xx  

### 7.2 手动

1. 默认配置：源站不可达 → CF 风格页，真实 502/504，页内数字一致  
2. 源站返回 503 → 替换页，status 503  
3. 仅标签 `522` → 仅 522 替换  
4. 关闭开关并发布 → 透传恢复  
5. 自定义 HTML 占位符预览与线上一致  
6. Pages 路由不受影响  

### 7.3 文档

* 本设计文档；`docs/design/index.md` 能力表；`docs/config.ts` 侧栏  
* changelog `[Unreleased]` 用户可读改进条  

---

## 8. 实现要点清单（供计划拆分）

1. goose seed 三个 Option key + model 常量  
2. 状态码解析/校验纯函数 + 单测  
3. Option update 路径挂接校验  
4. 快照填充 `ConfigSnapshot` 新字段  
5. `pkg/render/openresty`：error_page 块、SupportFile、默认 HTML、单测  
6. Agent 侧若需 Lua 辅助文件，随现有 nginx lua 目录同步  
7. 前端 Tags Input + `/error-pages` 页 + 导航  
8. changelog 与设计索引  

---

## 9. 决策记录

| 决策 | 选择 | 原因 |
| --- | --- | --- |
| 配置范围 | 全局 | 产品要求；实现与运维简单 |
| 存储 | Option + 配置版本 | 与性能调优一致，可回滚 |
| 状态码输入 | 标签：单码与区间 | 默认整段 5xx，又可点名 522 |
| 响应 status | 保持原码 | 监控/SEO/客户端语义正确 |
| 运行时替换 | internal + 轻量模板替换 | 每请求 status 不同 |
| 自定义方式 | 在线 HTML | 灵活且无需文件上传链路 |
`}