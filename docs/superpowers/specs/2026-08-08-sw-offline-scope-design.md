# SW 离线兜底生效范围（域名作用域）设计

- 日期：2026-08-08
- 状态：设计已确认
- 前置：issue #23 Service Worker 离线兜底（`docs/superpowers/specs/2026-08-08-service-worker-offline-design.md`）
- 范围：SW 注入从「全局所有 HTTPS 站点」细化为「总开关 + 域名作用域」

## 1. 背景与目标

issue #23 实现后，`sw_offline_enabled` 为全局布尔开关：开启后对所有启用 HTTPS 的路由注入 Service Worker 离线兜底。本需求将其细化为可选的**生效域名范围**：

- 保留总开关（`sw_offline_enabled`）。
- 新增作用域：管理员选择需要生效的域名，仅作用域内域名注入 SW。
- 域名选择交互参考 `/cloudflare/groups/1` 的「添加域名成员」弹窗（搜索筛选、按 Zone 分组、批量勾选），但**与 Cloudflare 完全解耦**——仅复用交互模式，数据源为平台自身 zones/zone_domains，不涉及 A 记录同步。

核心约束：

- 语义为「总开关 && 域名 ∈ 作用域」交集：总开关关 → 全部不注入；总开关开 + 作用域空 → 不注入；总开关开 + 域名命中 → 注入。
- 与 Cloudflare 指向分组（A 记录）无任何关联。
- 联系页 HTML（`sw_offline_html`）仍为全局单份，不分域名定制。

## 2. 机制总览

```
sw_offline_enabled  (bool,  已有)        总开关
sw_offline_html     (string, 已有)        联系页 HTML（全局一份）
sw_offline_domains  (JSON 字符串数组, 新增) 生效域名作用域

渲染: routeSWEnabled(routeDomains, cfg)
      = SWOfflineEnabled && routeDomains ∩ SWOfflineDomains ≠ ∅
      命中 → HTTPS server 块注入 access 检查 + SW location
      未命中 → 与 feature 前字节一致
```

Support files（`sw/sw.js`、`sw/offline.html`）仅在「总开关开 && 作用域非空」时下发，避免空作用域产生无用资源。

## 3. 数据层

### 3.1 配置 key

`model.ConfigKeySWOfflineDomains = "sw_offline_domains"`（business 类型，visibility 0），值存 JSON 域名字符串数组：

```json
["example.com", "api.example.com"]
```

### 3.2 goose 迁移（postgres + sqlite 各一份）

`INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at) VALUES ('sw_offline_domains', '[]', 'business', 0, 'SW 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT (key) DO NOTHING;`

Down 删除该 key。migrator 测试计数 92 → 93，并更新注释。

### 3.3 validator

`validateSWOfflineDomains(key, value string) error`，注册进 `openRestyOptionValidators`：

- JSON 解析为 `[]string`，失败报「必须为 JSON 字符串数组」
- 元素去重（重复报错）
- 元素非空、小写规范化校验（复用/对齐 zone `normalizeDomain` 的域名格式约束：无 `*`、无 `://` `/` `?` `#` `@`、`publicsuffix.EffectiveTLDPlusOne` 可解析）
- 数量上限 `maxSWOfflineDomains = 1000`（防滥用）

### 3.4 config_version snapshot

- `openRestyConfigSnapshot`（`snapshot.go`）新增 `SWOfflineDomains []string json:"sw_offline_domains,omitempty"`。
- `buildOpenRestyConfigSnapshot` 新增 `getStringSliceConfig(key string, defaultVal []string) []string`（解析 JSON 数组，失败回退默认），赋值 `SWOfflineDomains: getStringSliceConfig(model.ConfigKeySWOfflineDomains, nil)`。
- `logics.go`：`diffOpenRestyOptionDetails` 追加 `appendIfChanged("SWOfflineDomains", ...)`；`openRestyOptionKeys()` 追加 `"SWOfflineDomains"`。

## 4. 渲染层（pkg/render/openresty）

### 4.1 ConfigSnapshot

`types.go` 的 `ConfigSnapshot` 新增：

```go
// SWOfflineDomains restricts the offline fallback to matching HTTPS routes.
SWOfflineDomains []string `json:"sw_offline_domains,omitempty"`
```

### 4.2 作用域判断

```go
// routeSWEnabled returns true when SW offline fallback applies to this route.
func routeSWEnabled(routeDomains []string, cfg ConfigSnapshot) bool {
    if !cfg.SWOfflineEnabled || len(cfg.SWOfflineDomains) == 0 {
        return false
    }
    scope := make(map[string]struct{}, len(cfg.SWOfflineDomains))
    for _, d := range cfg.SWOfflineDomains {
        scope[d] = struct{}{}
    }
    for _, d := range routeDomains {
        if _, ok := scope[d]; ok {
            return true
        }
    }
    return false
}
```

域名精确匹配（存储时已小写规范化）。

### 4.3 server 渲染签名扩展

- `RenderRouteConfig`：每 route 计算 `swEnabled := routeSWEnabled(domains, doc.OpenRestyConfig)`，传入 `renderProxyRoute` / `renderPagesRoute`（新增 `swEnabled bool` 参数）。
- 下传链路：`renderProxyRouteHTTPS` / `renderPagesRouteHTTPS` / `renderHTTPSServer` / `renderHTTPSPagesServer` 均新增 `swEnabled bool` 参数。
- `swEnabled=true` → `renderAccessBlockWithSW(siteName, powEnabled, cfg)` + 追加 `renderServiceWorkerChallenger(cfg)`（现行为，两函数内部不再判断 `SWOfflineEnabled`，条件已上移到 route 层）。
- `swEnabled=false` → 纯 `renderAccessBlock`，无 challenger（与 feature 前字节一致）。
- HTTP（80）server 块保持不注入（issue #23 已定 HTTPS-only）。
- `renderAccessBlockWithSW` / `renderServiceWorkerChallenger` 保留 `cfg` 参数（HTML 内容来自 `cfg.SWOfflineHTML`），仅移除其内部开关判断。

### 4.4 Support files

`Render` 中生成条件从 `if doc.OpenRestyConfig.SWOfflineEnabled` 改为：

```go
if doc.OpenRestyConfig.SWOfflineEnabled && len(doc.OpenRestyConfig.SWOfflineDomains) > 0 {
    files = append(files, ServiceWorkerSupportFiles(doc.OpenRestyConfig)...)
}
```

### 4.5 测试

- `routeSWEnabled`：开关关 / 作用域空 / 无交集 / 单域名交集 / 多域名部分交集。
- HTTPS server 渲染：命中 → 含 `require("sw.runtime").check()` + 三个 SW location；未命中 → 与旧输出字节一致。
- `Render`：空作用域不下发 `sw/*` support files。
- 现有 `TestRenderAccessBlockWithSWMergesSingleBlock` 等适配新签名（`cfg` 语义变化：禁用时不再由内部判断，改由上层传 `swEnabled`）。

## 5. 前端（frontend/app/(main)/responses）

### 5.1 联系页 tab 布局

联系页 tab 两张卡片：

**卡片 1：离线兜底（总开关）**
- 标题「离线兜底」+ 描述。
- 右上角「保存」按钮。
- 「启用 Service Worker 离线兜底」Switch（`sw_offline_enabled`）。
- 「生效范围」区块：当前已选域名 badge 列表（可移除）+「添加域名」按钮打开弹窗；开关关闭时整卡禁用/置灰。
- 保存时 `updateBatch` 一次性提交三个 key：
  ```ts
  { key: KEY_SW_ENABLED, value: String(fields.enabled) },
  { key: KEY_SW_HTML, value: fields.html },
  { key: KEY_SW_DOMAINS, value: JSON.stringify(fields.domains) },
  ```
- 保存成功后 `invalidateResponseQueries`（toast 提示「请前往版本发布使配置生效」不变）。

**卡片 2：联系页 HTML**
- 复用 `HtmlEditorWorkspace`（见 5.3），无占位符，实时预览原样 HTML。

### 5.2 域名选择弹窗（scope-domain-dialog.tsx）

- 交互复用 `member-add-dialog.tsx`：搜索框（域名/zone 模糊匹配）、按 Zone 分组折叠、组内勾选/取消、全选可见/清空、已选计数。
- 无橙云开关、无 Cloudflare 依赖。
- 数据源：`ZoneService.list()` + 每 zone `ZoneService.getOverview(id)` 并行拉取（`Promise.all`），zone 根域并入对应分组。**不新增后端 API**。
- 弹窗预勾选当前已生效域名；确认后返回选中的域名字符串数组（覆盖式替换本地 fields.domains）。
- 空态：无 zone 时提示「暂无可用域名，请先在 Zone 管理中注册」。

### 5.3 HtmlEditorWorkspace 复用（泛化）

`frontend/app/(main)/error-pages/components/html-editor-workspace.tsx` 泛化并移至 `frontend/components/common/html-editor-workspace.tsx`：

- Props 扩展：
  - `maxBytes?: number`（默认 `ORIGIN_ERROR_PAGE_HTML_MAX_BYTES` = 256 KiB，SW 同为 256 KiB 常量可共用）
  - `preview?: (html: string) => string`（默认 `previewOriginErrorPageHTML`；SW 传 `(html) => html` 原样预览）
  - `footerHint?: React.ReactNode`（预览 footer 提示文案，默认错误页的「`{{status}}`→502 · `{{host}}`→example.com」；SW 传 `null`）
- 错误页 `edit/page.tsx` 改 import 路径，行为不变。
- `frontend/components/common/` 若不存在则创建目录。

### 5.4 shared.ts 与表单

- `KEY_SW_DOMAINS = 'sw_offline_domains'`。
- `ContactPageFields` 增加 `domains: string[]`；`defaultContactPageFields.domains = []`。
- `mapOptionsToContactFields` 解析 `sw_offline_domains` JSON（容错：非法 JSON → `[]`）。

## 6. 验证

- 后端：`go test ./pkg/render/openresty/... ./internal/apps/openflare/option/... ./internal/apps/openflare/config_version/... ./internal/infra/persistence/migrator/...`
- 前端：`pnpm tsc --noEmit` + `eslint`（联系页新字段/弹窗/多 zone 并行拉取）
- 全量：`go test ./...`、`make code-check`、`make format`
- `make swagger`：无新 API（验证无变更即可）

## 7. Changelog

`docs/changelog/index.md` `[Unreleased]` 更新 SW 条目：新增「可指定生效域名范围（仅对选中的 HTTPS 域名生效）」。

## 8. 已知边界

- 作用域存域名字符串数组：域名从 zone/zone_domain 改名后需手动同步作用域（与 `route.Domains` 精确匹配）。
- 联系页 HTML 全局单份，不分域名定制。
- 空作用域 + 总开关开 → 不注入（前端置灰提示先选域名）。
- 匹配为精确匹配，不跨子域通配（选 `example.com` 不自动覆盖 `api.example.com`，需显式加入）。
