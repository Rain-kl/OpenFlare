# SW 离线兜底生效范围 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 SW 离线兜底从「全局所有 HTTPS 站点」细化为「总开关 + 域名作用域」：管理员在联系页选择生效域名，仅作用域内域名注入 SW。

**Architecture:** 新增全局 Option `sw_offline_domains`（JSON 域名数组），经 config_version snapshot 进入 `ConfigSnapshot`；渲染层每 route 计算 `routeSWEnabled(routeDomains, cfg)` 交集，命中才在 HTTPS server 块注入 SW。前端联系页 tab 增加「生效范围」卡片与域名选择弹窗（复用 member-add-dialog 交互、数据源走现有 zones 接口），HTML 编辑器复用泛化后的 `HtmlEditorWorkspace`。

**Tech Stack:** Go 1.25+、Gin、GORM、goose（PG/SQLite）、Next.js App Router、TypeScript、shadcn/ui、TanStack Query、CodeMirror。

## Global Constraints

- 语义「总开关 && 域名 ∈ 作用域」：总开关关 → 全部不注入；总开关开 + 作用域空 → 不注入；总开关开 + 域名命中 → 注入。
- 与 Cloudflare 指向分组（A 记录同步）**完全无关**——仅复用「添加域名成员」弹窗的交互模式（搜索/批量勾选/按 Zone 分组）。
- 联系页 HTML（`sw_offline_html`）仍全局单份，不分域名定制。
- 作用域域名精确匹配，不跨子域通配。
- 遵循 AGENTS.md 分层：`apps → repository → model`；API 错误用 `response.Abort*`。
- API 变更后 `make swagger`；开发完成 `make code-check`；提交前 `make format`。
- 迁移：PostgreSQL 与 SQLite 各一份 goose SQL（`goose/postgres/`、`goose/sqlite/`），见 `database-migration` skill。
- 代码/配置变更写入 `docs/changelog/index.md` 的 `[Unreleased]`（中文，用户可读）。
- 配置 key 命名：小写 snake_case。
- 前端：根容器 `w-full`，外层 `py-6 px-1`；标题行 `flex items-center gap-2`；shadcn 用 `variant` + CSS 变量，业务 `className` 不硬编码颜色。
- 测试临时目录只用 `t.TempDir()`。

---

### Task 1: 后端配置 key + validator + goose 迁移

**Files:**
- Modify: `internal/model/system_configs.go:120-122`（SW 常量块后追加）
- Modify: `internal/apps/openflare/option/openresty_validators.go:17`（max 常量）、`:65-66`（map 注册）、`:71-81`（validateOpenRestyOption）、`:253-255`（追加函数）
- Modify: `internal/apps/openflare/option/openresty_validators_test.go`（追加测试）
- Create: `internal/infra/persistence/migrator/goose/postgres/202608080002_add_sw_offline_domains.sql`
- Create: `internal/infra/persistence/migrator/goose/sqlite/202608080002_add_sw_offline_domains.sql`
- Modify: `internal/infra/persistence/migrator/migrator_test.go:22-26`（计数 92 → 93）

**Interfaces:**
- Produces: 常量 `model.ConfigKeySWOfflineDomains = "sw_offline_domains"`；validator `validateSWOfflineDomains(key, value string) error`；DB seed 行。

- [ ] **Step 1: 追加 key 常量**

在 `system_configs.go` 的 SW 常量块（`ConfigKeySWOfflineHTML` 后）追加：

```go
	ConfigKeySWOfflineDomains = "sw_offline_domains" // 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）
```

- [ ] **Step 2: 追加 validator 与注册**

在 `openresty_validators.go` 顶部常量块追加：

```go
const (
	maxOriginErrorPageHTMLBytes = 256 << 10 // 256 KiB
	maxSWOfflineDomains         = 1000
)
```

（把现有单行 `const maxOriginErrorPageHTMLBytes = 256 << 10` 改为上方块。）

在 `openRestyOptionValidators` map（`:65-66`）追加：

```go
	model.ConfigKeySWOfflineDomains:                     validateSWOfflineDomains,
```

文件末尾追加实现（JSON 数组、去重、小写规范化、格式校验）：

```go
func validateSWOfflineDomains(key, value string) error {
	var domains []string
	if err := json.Unmarshal([]byte(value), &domains); err != nil {
		return fmt.Errorf("%s 必须为 JSON 字符串数组", key)
	}
	if len(domains) > maxSWOfflineDomains {
		return fmt.Errorf("%s 最多支持 %d 个域名", key, maxSWOfflineDomains)
	}
	seen := make(map[string]struct{}, len(domains))
	for _, raw := range domains {
		domain := strings.ToLower(strings.TrimSpace(raw))
		if domain == "" {
			return fmt.Errorf("%s 包含空域名", key)
		}
		if _, ok := seen[domain]; ok {
			return fmt.Errorf("%s 包含重复域名 %s", key, domain)
		}
		seen[domain] = struct{}{}
	}
	return nil
}
```

（域名格式已由 zone 侧 `normalizeDomain` 保证；此处只做 JSON 结构、去重、空值与上限校验。若需更强格式校验，复用 `zone.normalizeDomain` 语义需导出，当前保持最小校验并在计划 Self-Review 记录。）

- [ ] **Step 3: 写 validator 测试**

在 `openresty_validators_test.go` 追加：

```go
func TestValidateSWOfflineDomains(t *testing.T) {
	cases := []struct {
		name  string
		value string
		ok    bool
	}{
		{"empty array", `[]`, true},
		{"single", `["example.com"]`, true},
		{"multiple", `["example.com","api.example.com"]`, true},
		{"invalid json", `not-json`, false},
		{"empty element", `[""]`, false},
		{"duplicate", `["example.com","example.com"]`, false},
		{"whitespace dedup", `[" Example.com ","example.com"]`, false},
		{"over limit", fmt.Sprintf(`[%s]`, strings.Repeat(`"a.com",`, maxSWOfflineDomains)+`"a.com"`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSWOfflineDomains("sw_offline_domains", tc.value)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}
```

确认测试文件已有 `fmt`/`strings` import（若无则补）。

- [ ] **Step 4: 运行 option 测试**

Run: `go test ./internal/apps/openflare/option/...`
Expected: PASS

- [ ] **Step 5: 创建 postgres 迁移**

创建 `goose/postgres/202608080002_add_sw_offline_domains.sql`：

```sql
-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('sw_offline_domains', '[]', 'business', 0, 'SW 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key = 'sw_offline_domains';
```

- [ ] **Step 6: 创建 sqlite 迁移**

内容与 postgres 完全相同，复制到 `goose/sqlite/202608080002_add_sw_offline_domains.sql`。

- [ ] **Step 7: 更新 migrator 测试计数**

`migrator_test.go:26` `expectedMigratedSystemConfigCount` 92 → 93；`:22-25` 注释追加「SW 离线兜底生效域名的 1 项业务配置」。

- [ ] **Step 8: 运行迁移测试**

Run: `go test ./internal/infra/persistence/migrator/...`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/model/system_configs.go internal/apps/openflare/option/openresty_validators.go internal/apps/openflare/option/openresty_validators_test.go internal/infra/persistence/migrator/goose/postgres/202608080002_add_sw_offline_domains.sql internal/infra/persistence/migrator/goose/sqlite/202608080002_add_sw_offline_domains.sql internal/infra/persistence/migrator/migrator_test.go
git commit -m "feat(option): add sw offline domains scope option"
```

---

### Task 2: ConfigSnapshot 字段 + config_version snapshot 接线

**Files:**
- Modify: `pkg/render/openresty/types.go`（`ConfigSnapshot` 末尾，`SWOfflineHTML` 后）
- Modify: `internal/apps/openflare/config_version/snapshot.go:148`（struct 字段）、`:511-517`（helper 追加）、`:564` 后（build 赋值）
- Modify: `internal/apps/openflare/config_version/logics.go:541-542`（diff 追加）、`:610-611`（option keys 追加）

**Interfaces:**
- Consumes: `model.ConfigKeySWOfflineDomains`（Task 1）。
- Produces: `ConfigSnapshot.SWOfflineDomains []string`；`openRestyConfigSnapshot.SWOfflineDomains []string`；snapshot JSON 内含 `sw_offline_domains` 字段，参与 checksum。

- [ ] **Step 1: ConfigSnapshot 追加字段**

`pkg/render/openresty/types.go` `ConfigSnapshot` 末尾（`SWOfflineHTML` 后）追加：

```go
	// SWOfflineDomains restricts the offline fallback to matching HTTPS routes.
	SWOfflineDomains []string `json:"sw_offline_domains,omitempty"`
```

- [ ] **Step 2: snapshot struct 追加字段**

`internal/apps/openflare/config_version/snapshot.go` `openRestyConfigSnapshot`（`:148`，`SWOfflineHTML` 后）追加：

```go
	SWOfflineDomains           []string `json:"sw_offline_domains,omitempty"`
```

- [ ] **Step 3: build 读取配置**

在 `buildOpenRestyConfigSnapshot` 内 `getStringConfig` helper 定义后（`:517` 后）追加：

```go
	getStringSliceConfig := func(key string, defaultVal []string) []string {
		config, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return defaultVal
		}
		var values []string
		if err := json.Unmarshal([]byte(config.Value), &values); err != nil {
			return defaultVal
		}
		return values
	}
```

确认 snapshot.go 已有 `encoding/json` import（`buildSnapshotWAFDocument` 等处应已用）。

在 snapshot 字面量 `SWOfflineHTML` 赋值后追加：

```go
		SWOfflineDomains:           getStringSliceConfig(model.ConfigKeySWOfflineDomains, nil),
```

- [ ] **Step 4: diff 与 option keys 追加**

`logics.go` `diffOpenRestyOptionDetails`（`:542` 后）追加：

```go
	appendIfChanged("SWOfflineDomains", strings.Join(left.SWOfflineDomains, ","), strings.Join(right.SWOfflineDomains, ","))
```

确认 logics.go 已 import `strings`（未用则补）。

`openRestyOptionKeys()`（`:611` 后）追加：

```go
		"SWOfflineDomains",
```

- [ ] **Step 5: 运行测试**

Run: `go build ./... && go test ./internal/apps/openflare/config_version/... ./pkg/render/openresty/...`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add pkg/render/openresty/types.go internal/apps/openflare/config_version/snapshot.go internal/apps/openflare/config_version/logics.go
git commit -m "feat(openresty): add sw offline domains snapshot field"
```

---

### Task 3: 渲染层作用域注入

**Files:**
- Modify: `pkg/render/openresty/render.go:52-54`（support files 条件）、`:97-117`（RenderRouteConfig 计算 swEnabled）、`:278`（renderHTTPProxyServer 签名）、`:325`（renderHTTPPagesServer 签名）、`:333-343`（renderHTTPSServer 签名与注入）、`:345-355`（renderHTTPSPagesServer 签名与注入）、`:451-504`（renderAccessBlock / renderAccessBlockWithSW 去开关判断）
- Modify: `pkg/render/openresty/render_route.go:46-104`（renderPagesRouteHTTPS / renderProxyRouteHTTPS 签名）、`:106-151`（renderPagesRoute / renderProxyRoute 签名）
- Modify: `pkg/render/openresty/service_worker.go`（`renderServiceWorkerChallenger` 去开关判断）
- Modify: `pkg/render/openresty/service_worker_test.go`（适配 + 新增 routeSWEnabled 测试）
- Modify: `pkg/render/openresty/render_test.go` 或相关（如有 SW 相关断言适配）

**Interfaces:**
- Consumes: `ConfigSnapshot.SWOfflineDomains`（Task 2）。
- Produces: `routeSWEnabled(routeDomains []string, cfg ConfigSnapshot) bool`；`renderHTTPSServer`/`renderHTTPSPagesServer`/`renderProxyRouteHTTPS`/`renderPagesRouteHTTPS`/`renderProxyRoute`/`renderPagesRoute` 均新增 `swEnabled bool` 参数（置于 `cfg` 之前）。

- [ ] **Step 1: 写失败测试（routeSWEnabled）**

在 `service_worker_test.go` 追加：

```go
func TestRouteSWEnabled(t *testing.T) {
	cfgOff := ConfigSnapshot{SWOfflineEnabled: false, SWOfflineDomains: []string{"example.com"}}
	if routeSWEnabled([]string{"example.com"}, cfgOff) {
		t.Fatal("expected false when master switch off")
	}
	cfgEmpty := ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: nil}
	if routeSWEnabled([]string{"example.com"}, cfgEmpty) {
		t.Fatal("expected false when scope empty")
	}
	cfgHit := ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: []string{"example.com", "other.com"}}
	if !routeSWEnabled([]string{"api.example.com", "example.com"}, cfgHit) {
		t.Fatal("expected true on single domain intersection")
	}
	if routeSWEnabled([]string{"api.example.com", "third.com"}, cfgHit) {
		t.Fatal("expected false on no intersection")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/render/openresty/ -run TestRouteSWEnabled`
Expected: FAIL（`routeSWEnabled` 未定义）

- [ ] **Step 3: 实现 routeSWEnabled**

在 `service_worker.go` 追加：

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

- [ ] **Step 4: 更新 renderServiceWorkerChallenger 去开关判断**

`service_worker.go` `renderServiceWorkerChallenger` 删除开头两行：

```go
	if !cfg.SWOfflineEnabled {
		return ""
	}
```

（调用方保证仅在 swEnabled 时调用；函数签名与其余逻辑不变。）

- [ ] **Step 5: 更新 renderAccessBlockWithSW 去开关判断**

`render.go` `renderAccessBlockWithSW` 删除：

```go
	if !cfg.SWOfflineEnabled {
		return renderAccessBlock(siteName, powEnabled)
	}
```

- [ ] **Step 6: RenderRouteConfig 计算 swEnabled 并下传**

`render.go` `RenderRouteConfig` 循环内（`renderPagesRoute`/`renderProxyRoute` 调用处）改为：

```go
		swEnabled := routeSWEnabled(domains, doc.OpenRestyConfig)
		if normalizeRouteUpstreamType(route.UpstreamType) == routeUpstreamTypePages {
			if err := renderPagesRoute(&builder, route, displayName, serverNames, certificates, limitConfig, powEnabled, swEnabled, doc.OpenRestyConfig); err != nil {
				return "", err
			}
			continue
		}
		if err := renderProxyRoute(&builder, route, displayName, serverNames, certificates, cacheConfig, limitConfig, powEnabled, swEnabled, doc.OpenRestyConfig); err != nil {
			return "", err
		}
```

- [ ] **Step 7: 更新 render_route.go 签名链**

`renderPagesRoute` / `renderProxyRoute` 增加 `swEnabled bool` 参数（置于 `powEnabled` 与 `cfg` 之间），并将 `renderPagesRouteHTTPS(...)` / `renderProxyRouteHTTPS(...)` 调用处传入 `swEnabled`；两个 `HTTPS` 函数增加 `swEnabled bool` 参数并透传到 `renderHTTPSPagesServer` / `renderHTTPSServer`。HTTP 分支（`renderHTTPPagesServer` / `renderHTTPProxyServer` / `renderHTTPRedirectServer`）调用保持不变（HTTP 不注入 SW）。

- [ ] **Step 8: 更新 render.go 四个 server 函数**

- `renderHTTPSServer`：新增 `swEnabled bool` 参数（`cfg` 前），`renderAccessBlockWithSW(siteName, powEnabled, cfg)` 替换为：

```go
	if swEnabled {
		return fmt.Sprintf("server {\n    listen 443 ssl;\n%s    http2 on;\n    server_name %s;\n    ssl_certificate %s;\n    ssl_certificate_key %s;\n%s%s%s    location / {\n%s%s%s%s%s%s    }\n%s%s%s}\n\n", h3Listen, serverNames, certPath, keyPath, h3Header, renderAccessBlockWithSW(siteName, powEnabled, cfg), renderPowLocationBlocks(powEnabled), renderBasicAuthBlock(basicAuthEnabled, basicAuthUsername, basicAuthPassword), renderProxyHeaderBlock(originURL, originHost, customHeaders, upstreamConfig, cfg), renderRouteLimitBlock(limitConfig), renderRouteCacheBlock(cacheConfig, cfg), renderOriginErrorPageIntercept(cfg), renderProxyPassBlock(originURL, upstreamConfig), renderOriginErrorPageServerBits(cfg), renderPowStaticLocationBlock(powEnabled), renderServiceWorkerChallenger(cfg))
	}
	return fmt.Sprintf("server {\n    listen 443 ssl;\n%s    http2 on;\n    server_name %s;\n    ssl_certificate %s;\n    ssl_certificate_key %s;\n%s%s%s    location / {\n%s%s%s%s%s%s    }\n%s%s}\n\n", h3Listen, serverNames, certPath, keyPath, h3Header, renderAccessBlock(siteName, powEnabled), renderPowLocationBlocks(powEnabled), renderBasicAuthBlock(basicAuthEnabled, basicAuthUsername, basicAuthPassword), renderProxyHeaderBlock(originURL, originHost, customHeaders, upstreamConfig, cfg), renderRouteLimitBlock(limitConfig), renderRouteCacheBlock(cacheConfig, cfg), renderOriginErrorPageIntercept(cfg), renderProxyPassBlock(originURL, upstreamConfig), renderOriginErrorPageServerBits(cfg), renderPowStaticLocationBlock(powEnabled))
```

（即：`swEnabled=true` 时 `renderAccessBlockWithSW` + challenger；`false` 时 `renderAccessBlock` 无 challenger。）

- `renderHTTPSPagesServer`：同样处理（pages 模板，challenger 追加位置在 `renderPowStaticLocationBlock(powEnabled)` 之后）。
- `renderHTTPProxyServer` / `renderHTTPPagesServer`：新增 `swEnabled bool` 参数（`cfg` 前），内部**不使用**该参数（HTTP 不注入）——用 `_ bool` 命名避免 lint 报错（参考现有 `renderHTTPPagesServer` 的 `_ ConfigSnapshot` 模式）。

- [ ] **Step 9: Render support files 条件**

`render.go:52-54` 改为：

```go
	if doc.OpenRestyConfig.SWOfflineEnabled && len(doc.OpenRestyConfig.SWOfflineDomains) > 0 {
		files = append(files, ServiceWorkerSupportFiles(doc.OpenRestyConfig)...)
	}
```

- [ ] **Step 10: 更新测试适配新签名**

在 `service_worker_test.go`：
- `TestRenderAccessBlockWithSWMergesSingleBlock`（`:95`、`:117`）改为直接测 `renderAccessBlockWithSW("example.com", powEnabled, ConfigSnapshot{})` 不再依赖内部开关——`renderAccessBlockWithSW` 现在无条件走合并逻辑，禁用语义由上层 `swEnabled` 控制。将两个 case 改为断言合并块内容（pow 分支含 waf/pow/sw 顺序；非 pow 分支含 waf/sw）。
- `TestRenderServiceWorkerChallenger`（`:159-162`）：删除「disabled 返回空串」断言（函数已无条件输出 location），改为仅断言启用输入输出含三个 location 与 `content_by_lua`。
- 新增「renderHTTPSServer 作用域命中/未命中」测试：构造 cfg（`SWOfflineEnabled: true` + domains 含/不含 route 域名），断言命中输出含 `require("sw.runtime").check()` 与 `location = /sw.js`，未命中输出不含。
- 检查 `render_test.go` 中直接调用 `renderHTTPSServer`/`renderHTTPSPagesServer`/`renderPagesRoute`/`renderProxyRoute` 的测试，逐一补 `swEnabled` 参数（未命中场景传 `false`，保持字节不变断言）。

- [ ] **Step 11: 运行测试**

Run: `go test ./pkg/render/openresty/...`
Expected: PASS（含全部适配与新增）

- [ ] **Step 12: 提交**

```bash
git add pkg/render/openresty/render.go pkg/render/openresty/render_route.go pkg/render/openresty/service_worker.go pkg/render/openresty/service_worker_test.go pkg/render/openresty/render_test.go
git commit -m "feat(openresty): scope sw offline injection by route domains"
```

---

### Task 4: HtmlEditorWorkspace 泛化与复用

**Files:**
- Create: `frontend/components/common/html-editor-workspace.tsx`（从 error-pages 复制并泛化）
- Delete: `frontend/app/(main)/error-pages/components/html-editor-workspace.tsx`
- Modify: `frontend/app/(main)/error-pages/edit/page.tsx:43`（import 路径）、`:122`（调用处适配可选 props）

**Interfaces:**
- Consumes: `ORIGIN_ERROR_PAGE_HTML_MAX_BYTES`、`previewOriginErrorPageHTML`（`@/lib/openflare/default-origin-error-page-html`，已存在）。
- Produces: `HtmlEditorWorkspace`（props: `value`, `onChange`, `toolbarRight?`, `maxBytes?` 默认 `ORIGIN_ERROR_PAGE_HTML_MAX_BYTES`, `preview?` 默认 `previewOriginErrorPageHTML`, `footerHint?` 默认错误页提示）。

- [ ] **Step 1: 创建 common 版本**

将 `error-pages/components/html-editor-workspace.tsx` **全文复制**到 `frontend/components/common/html-editor-workspace.tsx`，然后按以下 5 处改造（其余代码保持原样）：

**改造 1** — props 类型与解构签名替换为：

```tsx
type HtmlEditorWorkspaceProps = {
  value: string;
  onChange: (value: string) => void;
  toolbarRight?: React.ReactNode;
  maxBytes?: number;
  preview?: (html: string) => string;
  footerHint?: React.ReactNode;
};

export function HtmlEditorWorkspace({
  value,
  onChange,
  toolbarRight,
  maxBytes = ORIGIN_ERROR_PAGE_HTML_MAX_BYTES,
  preview = previewOriginErrorPageHTML,
  footerHint = (
    <>
      {'{{status}}'}→502 · {'{{host}}'}→example.com
    </>
  ),
}: HtmlEditorWorkspaceProps) {
```

**改造 2** — 原 `const previewSrcDoc = previewOriginErrorPageHTML(value);` 替换为：

```tsx
  const previewSrcDoc = preview(value);
```

**改造 3** — 原 `const htmlBytes = new TextEncoder().encode(value).length;` 保持不变，但顶部字节统计处 `{ORIGIN_ERROR_PAGE_HTML_MAX_BYTES}` 替换为 `{maxBytes}`。

**改造 4** — 预览 footer 中「`{{status}}`→502 · `{{host}}`→example.com」的 span 替换为 `{footerHint}`（`footerHint` 为 `null` 时用 `{footerHint ?? null}` 渲染空，且外层 span 在 `footerHint === null` 时加 `hidden` class——保持布局稳定）。

**改造 5** — 组件注释改为：`/** 通用 HTML 编辑器工作区：左代码 / 右实时预览横向布局，可拖拽分隔。 */`

- [ ] **Step 2: 更新错误页 import**

`edit/page.tsx`：

```ts
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
```

删除旧相对 import；调用处不变（可选 props 有默认值）。

- [ ] **Step 3: 删除旧组件**

`rm frontend/app/(main)/error-pages/components/html-editor-workspace.tsx`

确认 `error-pages/components/` 下无其他文件引用它（grep `html-editor-workspace`）。

- [ ] **Step 4: 联系页接入**

本任务只负责泛化与错误页替换；联系页接入在 Task 5 Step 3 完成（见下）。此步无需改动 contact-page-tab.tsx。

- [ ] **Step 5: 类型检查**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad/frontend && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add "frontend/components/common/html-editor-workspace.tsx" "frontend/app/(main)/error-pages/edit/page.tsx" "frontend/app/(main)/error-pages/components/html-editor-workspace.tsx"
git commit -m "refactor(frontend): generalize html editor workspace for reuse"
```

---


### Task 5: 前端 shared + 生效范围卡片 + 域名选择弹窗

**Files:**
- Modify: `frontend/app/(main)/responses/components/shared.ts`
- Modify: `frontend/app/(main)/responses/components/contact-page-tab.tsx`
- Create: `frontend/app/(main)/responses/components/scope-domain-dialog.tsx`

**Interfaces:**
- Consumes: `OptionService.list()/updateBatch()`（已存在）、`ZoneService.list()`/`ZoneService.getOverview(id)`（已存在）、`zoneQueryKey`（已存在）。
- Produces: `KEY_SW_DOMAINS`；`ContactPageFields.domains: string[]`；`ScopeDomainDialog` 组件（props: `open`, `onOpenChange`, `zones: ScopeZoneGroup[]`, `selected: string[]`, `pending`, `onSubmit(domains: string[])`；`ScopeZoneGroup = { zoneDomain: string; domains: string[] }`）。

- [ ] **Step 1: shared.ts 扩展**

```ts
export const KEY_SW_DOMAINS = 'sw_offline_domains';

export type ContactPageFields = {
  enabled: boolean;
  html: string;
  domains: string[];
};

export const defaultContactPageFields: ContactPageFields = {
  enabled: false,
  html: '',
  domains: [],
};

function parseDomains(raw: string | undefined): string[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === 'string')
      : [];
  } catch {
    return [];
  }
}
```

`mapOptionsToContactFields` 返回值追加 `domains: parseDomains(optionMap[KEY_SW_DOMAINS])`。

- [ ] **Step 2: 域名选择弹窗 scope-domain-dialog.tsx**

参考 `frontend/app/(main)/cloudflare/components/member-add-dialog.tsx` 的交互（搜索、按 Zone 分组折叠、组内勾选、全选可见/清空、已选计数、空态），但：

- 无橙云 Switch、无 Cloudflare 依赖。
- 数据源为 `{ zoneDomain: string; domains: { domain: string }[] }` 分组（zone 根域并入）。
- 预勾选：打开时以当前已生效域名初始化 `selected`（`Set<string>`）。
- 确认回调 `onSubmit([...selected])`（域名字符串数组）。

组件骨架（沿用 member-add-dialog 的 Dialog 结构，`sw` 前缀类名无冲突）：

```tsx
'use client';

import { useEffect, useMemo, useState } from 'react';
import { Check, ChevronDown, Search } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

type ScopeZoneGroup = {
  zoneDomain: string;
  domains: string[];
};

export function ScopeDomainDialog({
  open,
  onOpenChange,
  zones,
  selected,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  zones: ScopeZoneGroup[];
  selected: string[];
  pending: boolean;
  onSubmit: (domains: string[]) => void;
}) {
  const [keyword, setKeyword] = useState('');
  const [selectedSet, setSelectedSet] = useState<Set<string>>(() => new Set());
  const [collapsedZones, setCollapsedZones] = useState<Set<string>>(
    () => new Set(),
  );

  useEffect(() => {
    if (!open) return;
    setKeyword('');
    setSelectedSet(new Set(selected));
    setCollapsedZones(new Set());
  }, [open, selected]);

  const filtered = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return zones;
    return zones
      .map((group) => ({
        zoneDomain: group.zoneDomain,
        domains: group.domains.filter(
          (d) =>
            d.toLowerCase().includes(normalized) ||
            group.zoneDomain.toLowerCase().includes(normalized),
        ),
      }))
      .filter((group) => group.domains.length > 0);
  }, [zones, keyword]);

  const visibleDomains = useMemo(
    () => filtered.flatMap((group) => group.domains),
    [filtered],
  );
  const allVisibleSelected =
    visibleDomains.length > 0 &&
    visibleDomains.every((d) => selectedSet.has(d));
  const someVisibleSelected =
    visibleDomains.some((d) => selectedSet.has(d)) && !allVisibleSelected;

  const toggleOne = (domain: string) => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      if (next.has(domain)) next.delete(domain);
      else next.add(domain);
      return next;
    });
  };

  const toggleGroup = (domains: string[]) => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      const allSelected = domains.every((d) => next.has(d));
      for (const d of domains) {
        if (allSelected) next.delete(d);
        else next.add(d);
      }
      return next;
    });
  };

  const toggleAllVisible = () => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const d of visibleDomains) next.delete(d);
      } else {
        for (const d of visibleDomains) next.add(d);
      }
      return next;
    });
  };

  const toggleCollapsed = (zoneDomain: string) => {
    setCollapsedZones((prev) => {
      const next = new Set(prev);
      if (next.has(zoneDomain)) next.delete(zoneDomain);
      else next.add(zoneDomain);
      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>选择生效域名</DialogTitle>
          <DialogDescription>
            仅对选中的 HTTPS 域名注入离线兜底；搜索筛选与批量勾选。
          </DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='sw-scope-search'>域名</FieldLabel>
            <div className='space-y-2'>
              <div className='relative'>
                <Search className='pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  id='sw-scope-search'
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder='搜索域名或顶级域…'
                  className='pl-8'
                  disabled={pending}
                />
              </div>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                  <Badge variant='secondary' className='font-normal'>
                    已选 {selectedSet.size}
                  </Badge>
                  <span>可见 {visibleDomains.length}</span>
                </div>
                <div className='flex items-center gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || visibleDomains.length === 0}
                    onClick={toggleAllVisible}
                  >
                    {allVisibleSelected ? '取消全选' : '全选可见'}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || selectedSet.size === 0}
                    onClick={() => setSelectedSet(new Set())}
                  >
                    清空
                  </Button>
                </div>
              </div>
              {zones.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  暂无可用域名，请先在 Zone 管理中注册域名。
                </div>
              ) : filtered.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  没有匹配的域名
                </div>
              ) : (
                <div className='max-h-72 space-y-1 overflow-y-auto rounded-lg border p-2'>
                  {filtered.map((group) => {
                    const groupSelected = group.domains.filter((d) =>
                      selectedSet.has(d),
                    );
                    const allSelected = groupSelected.length === group.domains.length;
                    const someSelected = groupSelected.length > 0 && !allSelected;
                    const open = !collapsedZones.has(group.zoneDomain);
                    return (
                      <Collapsible
                        key={group.zoneDomain}
                        open={open}
                        onOpenChange={() => toggleCollapsed(group.zoneDomain)}
                      >
                        <div
                          className={cn(
                            'rounded-md',
                            (allSelected || someSelected) && 'bg-muted/30',
                          )}
                        >
                          <div className='flex items-center gap-1 px-1 py-0.5'>
                            <Checkbox
                              checked={allSelected ? true : someSelected ? 'indeterminate' : false}
                              disabled={pending}
                              onCheckedChange={() => toggleGroup(group.domains)}
                              aria-label={`选择顶级域 ${group.zoneDomain}`}
                              className='ml-1'
                            />
                            <CollapsibleTrigger asChild>
                              <button
                                type='button'
                                className='flex min-w-0 flex-1 items-center gap-2 rounded-md px-1.5 py-1.5 text-left text-sm font-medium hover:bg-muted/60'
                              >
                                <ChevronDown
                                  className={cn(
                                    'size-4 shrink-0 text-muted-foreground transition-transform',
                                    !open && '-rotate-90',
                                  )}
                                />
                                <span className='truncate'>{group.zoneDomain}</span>
                                <Badge
                                  variant='outline'
                                  className='ml-auto shrink-0 font-normal text-[10px]'
                                >
                                  {groupSelected.length}/{group.domains.length}
                                </Badge>
                              </button>
                            </CollapsibleTrigger>
                          </div>
                          <CollapsibleContent>
                            <div className='ml-4 space-y-0.5 border-l border-border/70 py-0.5 pl-2'>
                              {group.domains.map((domain) => {
                                const checked = selectedSet.has(domain);
                                const isApex = domain === group.zoneDomain;
                                return (
                                  <label
                                    key={domain}
                                    className={cn(
                                      'flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/60',
                                      checked && 'bg-muted/40',
                                    )}
                                  >
                                    <Checkbox
                                      checked={checked}
                                      disabled={pending}
                                      onCheckedChange={() => toggleOne(domain)}
                                    />
                                    <span className='min-w-0 flex-1 truncate text-sm'>
                                      {domain}
                                    </span>
                                    {isApex ? (
                                      <Badge
                                        variant='secondary'
                                        className='shrink-0 text-[10px] font-normal'
                                      >
                                        顶级域
                                      </Badge>
                                    ) : null}
                                    {checked ? (
                                      <Check className='size-3.5 shrink-0 text-primary' />
                                    ) : null}
                                  </label>
                                );
                              })}
                            </div>
                          </CollapsibleContent>
                        </div>
                      </Collapsible>
                    );
                  })}
                </div>
              )}
            </div>
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            取消
          </Button>
          <Button
            disabled={pending}
            onClick={() => onSubmit([...selectedSet].sort())}
          >
            确定
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 3: contact-page-tab.tsx 扩展**

- 移除 `Textarea` import（HTML 编辑器移到卡片 2）。
- 引入：

```tsx
import { X, Plus } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { ZoneService, zoneQueryKey } from '@/lib/services/openflare';
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import { ScopeDomainDialog } from './scope-domain-dialog';
import { KEY_SW_DOMAINS } from './shared';
```

- 状态：`const [scopeOpen, setScopeOpen] = useState(false);`。
- zones 查询（admin 已由父页 gate）：

```tsx
const zonesQuery = useQuery({
  queryKey: zoneQueryKey,
  queryFn: async () => {
    const zones = await ZoneService.list();
    const overviews = await Promise.all(
      zones.map((zone) => ZoneService.getOverview(zone.id)),
    );
    return overviews.map((ov) => ({
      zoneDomain: ov.zone.domain,
      domains: [ov.zone.domain, ...ov.domains.map((d) => d.domain)],
    }));
  },
});
```

- saveMutation 的 updateBatch 追加第三个 key：

```tsx
{ key: KEY_SW_DOMAINS, value: JSON.stringify(fields.domains) },
```

- 布局改为两张卡片：
  - 卡片 1「离线兜底」：Switch + 生效范围区块（badge 列表 + 移除按钮 + 「添加域名」按钮），`disabled={!fields.enabled}` 时置灰；HTML Textarea 部分删除。
  - 卡片 2「联系页 HTML」：`<HtmlEditorWorkspace value={fields.html} onChange={(v) => setFields((prev) => ({ ...prev, html: v }))} preview={(html) => html} footerHint={null} />`，`disabled={!fields.enabled}` 时置灰（外层 `pointer-events-none opacity-60` 包裹）。
  - 生效范围区块骨架：

```tsx
<div className='space-y-2'>
  <div className='flex items-center justify-between gap-3'>
    <div>
      <Label className='text-sm font-medium'>生效范围</Label>
      <p className='text-sm text-muted-foreground'>
        仅对选中的 HTTPS 域名生效；留空则不注入。
      </p>
    </div>
    <Button
      type='button'
      size='sm'
      variant='outline'
      disabled={!fields.enabled || pending}
      onClick={() => setScopeOpen(true)}
    >
      <Plus className='size-3.5' />
      添加域名
    </Button>
  </div>
  {fields.domains.length === 0 ? (
    <p className='text-sm text-muted-foreground'>
      {fields.enabled ? '尚未选择生效域名，保存后不注入任何站点。' : '启用离线兜底后可选择生效域名。'}
    </p>
  ) : (
    <div className='flex flex-wrap gap-2'>
      {fields.domains.map((domain) => (
        <Badge key={domain} variant='secondary' className='gap-1 font-normal'>
          {domain}
          <button
            type='button'
            className='hover:text-destructive'
            disabled={!fields.enabled}
            aria-label={`移除 ${domain}`}
            onClick={() =>
              setFields((prev) => ({
                ...prev,
                domains: prev.domains.filter((d) => d !== domain),
              }))
            }
          >
            <X className='size-3' />
          </button>
        </Badge>
      ))}
    </div>
  )}
</div>
```

  - 卡片 1 的 CardContent 中 HTML 部分移除后，将新生效范围区块放在 Switch 下方。
  - 渲染 `<ScopeDomainDialog open={scopeOpen} onOpenChange={setScopeOpen} zones={zonesQuery.data ?? []} selected={fields.domains} pending={saveMutation.isPending} onSubmit={(domains) => { setFields((prev) => ({ ...prev, domains })); setScopeOpen(false); }} />`。

- [ ] **Step 4: 类型检查**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad/frontend && npx tsc --noEmit`
Expected: PASS（`HtmlEditorWorkspace` 由 Task 4 提供，按序执行即可）

- [ ] **Step 5: 提交**

```bash
git add "frontend/app/(main)/responses/components/shared.ts" "frontend/app/(main)/responses/components/contact-page-tab.tsx" "frontend/app/(main)/responses/components/scope-domain-dialog.tsx"
git commit -m "feat(frontend): add sw scope domain picker and contact page fields"
```

---

### Task 6: Changelog 与收尾验证

**Files:**
- Modify: `docs/changelog/index.md`

- [ ] **Step 1: 更新 changelog**

`docs/changelog/index.md` `[Unreleased]` 下找到 SW 离线兜底条目，更新为包含作用域能力（保持单条、中文、用户可读、说明效果）：

```markdown
- 支持 Service Worker 离线兜底：为启用 HTTPS 的网站下发 Service Worker 并缓存离线联系页，域名无法访问时浏览器展示联系站长页面，减少用户流失。可指定生效域名范围（仅对选中的 HTTPS 域名生效），配置位于「响应页面」-「联系页」，可在版本发布中批量生效。
```

- [ ] **Step 2: 全量验证**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad && make swagger && make code-check && go test ./... && make format`
Expected: 全 PASS（`go test` 若有已知 flaky 测试如 `TestStopCancelsRunningProcesses`，重跑确认非本分支引入）

- [ ] **Step 3: 提交**

```bash
git add docs/changelog/index.md
git commit -m "docs: sw offline scope changelog"
```

---

## Self-Review

**Spec 覆盖检查：**
- 数据层 key/迁移/validator/snapshot → Task 1、2 ✓
- 渲染层 routeSWEnabled + 签名链 + support files 条件 → Task 3 ✓
- 前端生效范围卡片 + 域名弹窗 + shared/保存 → Task 5 ✓
- HtmlEditorWorkspace 泛化复用 → Task 4 ✓
- Changelog + 全量验证 → Task 6 ✓

**占位符扫描：** 无 TBD/TODO；所有代码块完整可抄。

**类型一致性：**
- `routeSWEnabled([]string, ConfigSnapshot) bool` 在 Task 3 定义与使用一致。
- `swEnabled bool` 参数在 `renderProxyRoute`/`renderPagesRoute`/`renderProxyRouteHTTPS`/`renderPagesRouteHTTPS`/`renderHTTPSServer`/`renderHTTPSPagesServer`/`renderHTTPProxyServer`/`renderHTTPPagesServer` 全部一致（`cfg` 前）。
- `KEY_SW_DOMAINS`/`ContactPageFields.domains` 在 Task 5 一致。
- `HtmlEditorWorkspace` 新 props（`maxBytes`/`preview`/`footerHint`）在 Task 4 定义与 Task 5 使用一致。

**已知边界（记录在案）：**
- Task 1 validator 仅做 JSON 结构/去重/空值/上限校验，未做域名格式校验（zone 侧已保证来源合法；作用域 value 可能被手改 API 写入，格式校验留作后续增强——如需强化可复用 zone 包逻辑，但会引入跨包依赖，当前保持最小校验）。
- Task 3 Step 8 中 `renderHTTPSServer` 的双分支实现为「if swEnabled 返回 A 否则返回 B」；两分支 fmt 模板需保持完全一致（除 access/challenger 部分），测试断言未命中输出与 feature 前字节一致。
