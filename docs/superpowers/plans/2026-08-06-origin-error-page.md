# 源站错误页 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全局可配置源站/网关错误页：默认标签 `500-599`、Cloudflare 风格 HTML、可在线自定义；HTTP 状态码保持原值并在页面展示 `{{status}}`/`{{host}}`；可关闭恢复透传。

**Architecture:** 三个 Option key 持久化 → 配置版本 `openresty_config` 快照 → `pkg/render/openresty` 对反代 server 生成 `proxy_intercept_errors` + `error_page` + internal location；模板 SupportFile + 轻量 `content_by_lua_block` 替换占位符。管理端 `/error-pages` 挂在侧栏「网站管理」。

**Tech Stack:** Go、goose、Option API、`pkg/render/openresty`、OpenResty/Lua、Next.js、Tags Input、OptionService

**Spec:** [docs/design/origin-error-page.md](../../design/origin-error-page.md) · [docs/superpowers/specs/2026-08-06-origin-error-page-design.md](../specs/2026-08-06-origin-error-page-design.md)

## Global Constraints

- 仅**反代**路由应用；**Pages** 路由不生成错误页指令
- HTTP **status 保持原错误码**，禁止统一改为 200
- 状态码标签：单码 `522` 或闭区间 `500-599`；范围 **400–599**；默认 `["500-599"]`
- 占位符：`{{status}}`、`{{host}}`；HTML 空 = 内置默认；最大 **256 KiB**
- 保存走 Option `update-batch`；**需配置版本发布**后边缘生效
- 完成后 `make code-check`；前端相关 `make format` / prettier；中文 changelog；不写英文文档
- 不新增独立业务路由注册（复用 `/api/v1/d/option`）；侧栏仅前端导航

## File map

| 文件 | 职责 |
|------|------|
| `pkg/render/openresty/status_codes.go` | 标签解析/展开纯函数 |
| `pkg/render/openresty/status_codes_test.go` | 解析单测 |
| `pkg/render/openresty/origin_error_page.go` | 默认 HTML、渲染 error_page 片段、SupportFile 路径常量 |
| `pkg/render/openresty/origin_error_page_test.go` | 渲染片段单测 |
| `pkg/render/openresty/types.go` | `ConfigSnapshot` 三字段 |
| `pkg/render/openresty/render.go` / `render_route.go` | 接入 error 块到反代 server |
| `pkg/render/openresty/render_test.go` | 集成渲染断言 |
| `internal/model/system_configs.go` | 三个 ConfigKey 常量 |
| `internal/infra/persistence/migrator/goose/{postgres,sqlite}/202608060001_add_origin_error_page_options.sql` | seed |
| `internal/apps/openflare/option/openresty_validators.go` | 校验 enabled / codes / html |
| `internal/apps/openflare/option` 相关 test | 校验失败用例 |
| `internal/apps/openflare/config_version/snapshot.go` | 快照读写字段 |
| `internal/apps/openflare/config_version/logics.go` | `diffOpenRestyOptionDetails` 含新字段 |
| `frontend/components/ui/tags-input.tsx` | shadcn-extension Tags Input（若缺失则添加） |
| `frontend/app/(main)/error-pages/page.tsx` | 设置页 |
| `frontend/lib/navigation/openflare-nav.ts` | 网站管理菜单 |
| `frontend/lib/utils/search-data.ts` | 搜索 |
| `docs/reference/configuration.md` | 配置键说明（中文） |
| `docs/changelog/index.md` | Unreleased |
| `docs/plan/index.md` | 进行中索引 |

---

### Task 1: 状态码标签解析（纯函数 TDD）

**Files:**
- Create: `pkg/render/openresty/status_codes.go`
- Create: `pkg/render/openresty/status_codes_test.go`

**Interfaces:**
- Produces:
  - `func ExpandStatusCodeTags(tags []string) (codes []int, err error)`
  - `func ParseStatusCodeTag(tag string) (lo, hi int, err error)` — 单码时 `lo==hi`
  - 常量：`StatusCodeMin = 400`, `StatusCodeMax = 599`
- 规则：trim；`^\d{3}$` 或 `^\d{3}-\d{3}$`；`lo<=hi`；均在 400–599；展开 inclusive；排序去重；空 tags → 空 slice + nil error（「启用且空」由校验层拒绝）

- [ ] **Step 1: 写失败单测**

```go
func TestExpandStatusCodeTags(t *testing.T) {
	t.Parallel()
	codes, err := ExpandStatusCodeTags([]string{"500-502", "522", "501"})
	if err != nil {
		t.Fatal(err)
	}
	// want sorted unique: 500,501,502,522
	if len(codes) != 4 || codes[0] != 500 || codes[3] != 522 {
		t.Fatalf("got %v", codes)
	}
	_, err = ExpandStatusCodeTags([]string{"399"})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = ExpandStatusCodeTags([]string{"503-500"})
	if err == nil {
		t.Fatal("expected reverse range error")
	}
	_, err = ExpandStatusCodeTags([]string{"5xx"})
	if err == nil {
		t.Fatal("expected syntax error")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/render/openresty/ -run TestExpandStatusCodeTags -count=1`  
Expected: FAIL（函数未定义）

- [ ] **Step 3: 实现 `status_codes.go`**

```go
package openresty

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	StatusCodeMin = 400
	StatusCodeMax = 599
)

func ParseStatusCodeTag(tag string) (lo, hi int, err error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0, 0, fmt.Errorf("状态码标签不能为空")
	}
	if i := strings.IndexByte(tag, '-'); i >= 0 {
		lo, err = strconv.Atoi(tag[:i])
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码区间: %s", tag)
		}
		hi, err = strconv.Atoi(tag[i+1:])
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码区间: %s", tag)
		}
	} else {
		lo, err = strconv.Atoi(tag)
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码: %s", tag)
		}
		hi = lo
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("状态码区间左右端点反序: %s", tag)
	}
	if lo < StatusCodeMin || hi > StatusCodeMax {
		return 0, 0, fmt.Errorf("状态码须在 %d–%d: %s", StatusCodeMin, StatusCodeMax, tag)
	}
	return lo, hi, nil
}

func ExpandStatusCodeTags(tags []string) ([]int, error) {
	set := map[int]struct{}{}
	for _, tag := range tags {
		lo, hi, err := ParseStatusCodeTag(tag)
		if err != nil {
			return nil, err
		}
		for c := lo; c <= hi; c++ {
			set[c] = struct{}{}
		}
	}
	out := make([]int, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Ints(out)
	return out, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./pkg/render/openresty/ -run TestExpandStatusCodeTags -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/render/openresty/status_codes.go pkg/render/openresty/status_codes_test.go
git commit -m "feat(openresty): add status code tag expand helper"
```

---

### Task 2: ConfigSnapshot 字段 + 默认 HTML + error_page 渲染（TDD）

**Files:**
- Modify: `pkg/render/openresty/types.go` — `ConfigSnapshot` 增加：
  - `OriginErrorPageEnabled bool \`json:"origin_error_page_enabled"\``
  - `OriginErrorPageStatusCodes []string \`json:"origin_error_page_status_codes,omitempty"\``
  - `OriginErrorPageHTML string \`json:"origin_error_page_html,omitempty"\``
- Create: `pkg/render/openresty/origin_error_page.go` — 默认 HTML 常量、路径常量、`renderOriginErrorPageDirectives`、`originErrorPageSupportFile`
- Modify: `pkg/render/openresty/render.go` — `Render`/`RenderRouteConfig` 在 enabled 时 append SupportFile
- Modify: `pkg/render/openresty/render_route.go`（或 `render.go` 中 `renderHTTPProxyServer` / `renderHTTPSServer`）— 在 proxy location **与** server 级写入 intercept + error_page + internal location
- Test: `pkg/render/openresty/origin_error_page_test.go` + 扩展 `render_test.go`

**Interfaces:**
- Produces:
  - `const OriginErrorPageSupportPath = "error_pages/origin_error.html.tmpl"`
  - `const DefaultOriginErrorPageHTML = \`...\`` — CF 风格，含 `{{status}}` `{{host}}`
  - `func EffectiveOriginErrorPageHTML(cfg ConfigSnapshot) string` — 空则默认
  - `func renderOriginErrorPageServerBits(cfg ConfigSnapshot) string` — 若 disabled 或 expand 失败/空则 `""`；否则 `error_page ...` + internal location 字符串
  - SupportFile content = Effective HTML（保留占位符）

**Internal location 形状（必须 status 透传）：**

```nginx
    location = /__openflare_origin_error {
        internal;
        default_type text/html;
        charset utf-8;
        content_by_lua_block {
            local f = io.open("__OPENFLARE_ERROR_PAGE_TMPL__", "r")
            if not f then
                ngx.status = ngx.status
                ngx.say("Error ", ngx.status)
                return
            end
            local body = f:read("*a")
            f:close()
            local status = tostring(ngx.status)
            local host = ngx.var.host or ""
            body = body:gsub("{{status}}", status, 1)
            body = body:gsub("{{host}}", host, 1)
            -- 全局替换剩余占位（若模板多处 status）
            body = body:gsub("{{status}}", status)
            body = body:gsub("{{host}}", host)
            ngx.header["Content-Type"] = "text/html; charset=utf-8"
            ngx.say(body)
        }
    }
```

占位路径：渲染时用常量如 `ErrorPageTmplPlaceholder = "__OPENFLARE_ERROR_PAGE_TMPL__"`，Agent 落盘时与 SupportFile 绝对路径替换（若现有 Agent 已有 support file root 替换模式则复用；否则在 `internal/apps/agent` 同步路径处增加对该 placeholder 的替换，与 `CertDirPlaceholder` 同类）。

在每个反代 `location /` 内（proxy 块）：

```nginx
        proxy_intercept_errors on;
```

在 server 块内（location 外）：

```nginx
    error_page 500 501 ... = /__openflare_origin_error;
```

+ internal location。

Pages 的 `renderHTTPSPagesServer` / pages HTTP **不**调用。

- [ ] **Step 1: 写失败单测**

```go
func TestRenderOriginErrorPageEnabled(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{
			OriginErrorPageEnabled:     true,
			OriginErrorPageStatusCodes: []string{"500-599"},
		},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "proxy_intercept_errors on") {
		t.Fatal("missing intercept")
	}
	if !strings.Contains(out, "error_page") || !strings.Contains(out, "/__openflare_origin_error") {
		t.Fatal("missing error_page")
	}
	if !strings.Contains(out, "{{status}}") == false {
		// SupportFile 在 Render 全量结果中
	}
	res, err := Render(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.SupportFiles {
		if f.Path == OriginErrorPageSupportPath {
			found = true
			if !strings.Contains(f.Content, "{{status}}") {
				t.Fatal("template missing placeholder")
			}
		}
	}
	if !found {
		t.Fatal("missing support file")
	}
}

func TestRenderOriginErrorPageDisabled(t *testing.T) {
	t.Parallel()
	doc := Document{
		Routes: []Route{{
			ID: 1, SiteName: "ex", Domains: []string{"ex.test"},
			OriginURL: "http://127.0.0.1:9", Enabled: true,
		}},
		OpenRestyConfig: ConfigSnapshot{OriginErrorPageEnabled: false},
	}
	out, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "proxy_intercept_errors") {
		t.Fatal("should not intercept when disabled")
	}
}
```

- [ ] **Step 2: 运行确认失败** → 实现默认 HTML + 渲染接入 → 运行 PASS

默认 HTML 要求：大号 `{{status}}`、展示 `{{host}}`、中性「源站暂时无法提供服务」类文案、无 Cloudflare 商标、可单文件 inline CSS。

- [ ] **Step 3: Agent placeholder 替换**

搜索 Agent 写 conf 时如何替换 `__OPENFLARE_CERT_DIR__` 等，为 `__OPENFLARE_ERROR_PAGE_TMPL__` 增加指向 support 目录下 `error_pages/origin_error.html.tmpl` 的绝对路径。  
若 conf 内 lua 无法可靠 `io.open` 绝对路径，可改为 `content_by_lua_file` + 小 lua 读固定相对路径；优先与现有 pow/waf 资源部署方式一致。

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(openresty): render origin error page directives"
```

---

### Task 3: Option keys、迁移、校验、快照

**Files:**
- Modify: `internal/model/system_configs.go` — 常量：
  - `ConfigKeyOriginErrorPageEnabled = "origin_error_page_enabled"`
  - `ConfigKeyOriginErrorPageStatusCodes = "origin_error_page_status_codes"`
  - `ConfigKeyOriginErrorPageHTML = "origin_error_page_html"`
- Create goose（postgres + sqlite 同名版本号）`202608060001_add_origin_error_page_options.sql`：

```sql
-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('origin_error_page_enabled', 'true', 'business', 0, '是否启用源站错误页', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('origin_error_page_status_codes', '["500-599"]', 'business', 0, '源站错误页触发状态码标签 JSON 数组', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('origin_error_page_html', '', 'business', 0, '源站错误页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'origin_error_page_enabled',
  'origin_error_page_status_codes',
  'origin_error_page_html'
);
```

（SQLite 若无 `ON CONFLICT` 同现有迁移方言对齐。）

- Modify: `openresty_validators.go`：
  - enabled → `validateBooleanOption`
  - status_codes → JSON 数组 `[]string`，对每项 `ParseStatusCodeTag`；若 `enabled==true`（跨 key 时可用 state，或在 batch 校验后单独检查：若本 key 合法但 enabled 为 true 且 expand 为空则失败）。**实现建议**：`validateOriginErrorPageStatusCodes` 只校验标签可解析且 expand 非空（即使 disabled 也要求非空列表，避免脏数据）；html 长度 `<= 256*1024`
  - html → `len(value) <= 256<<10`

- Modify: `snapshot.go` 的 `openRestyConfigSnapshot` + `buildOpenRestyConfigSnapshot`：
  - Enabled: `getBoolConfig(..., true)`
  - StatusCodes: 解析 JSON 数组，失败则默认 `[]string{"500-599"}`
  - HTML: `getStringConfig(..., "")`

- Modify: `logics.go` `diffOpenRestyOptionDetails` 比较新字段（否则发布 diff 不显示）

- [ ] **Step 1: 校验单测**（`option` 包）非法 `["abc"]`、超大 html、合法 `["522","500-502"]`

- [ ] **Step 2: 实现迁移与 snapshot 填充**

- [ ] **Step 3: 手动或单测确认 snapshot JSON 含字段**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(option): seed and validate origin error page options"
```

---

### Task 4: 前端 Tags Input + `/error-pages` 页

**Files:**
- Create/Modify: `frontend/components/ui/tags-input.tsx`（若无：用 shadcn skill / `npx shadcn@latest add` 社区 Tags Input；API：`value: string[]`, `onChange`, `placeholder`）
- Create: `frontend/app/(main)/error-pages/page.tsx`（及可选 `components/` 拆分若超 600 行）
- Modify: `frontend/lib/navigation/openflare-nav.ts` — `openflareWebsiteNavGroup.items` 增加 `{ title: '错误页', url: '/error-pages' }`；`openflareWebsiteSubNav` 同步
- Modify: `frontend/lib/utils/search-data.ts` — 关键词：错误页、源站、502、error page
- Reuse: `OptionService.list` / `updateBatch`（同 performance 页）

**UI 行为：**
1. 管理员加载 options → 映射三字段  
2. Switch 启用  
3. TagsInput：默认展示解析后的 JSON 数组  
4. Textarea HTML；按钮「加载默认模板」「恢复默认」  
5. 预览：客户端把 `{{status}}`→`502`、`{{host}}`→`example.com` 后 iframe `srcDoc`  
6. 保存：`updateBatch` 三条；toast 提示去版本发布  
7. 前端校验：标签用与后端相同规则（可抽 `lib/openflare/status-code-tags.ts` 轻量实现或仅保存时依赖后端错误）

默认 HTML：从前端常量复制与 Go `DefaultOriginErrorPageHTML` **内容一致**（计划实现时两处同一字符串；可在 PR 说明需人工对齐）。

- [ ] **Step 1: 添加 Tags Input 组件并在 demo 或本页使用**

- [ ] **Step 2: 实现 page.tsx**

- [ ] **Step 3: 导航与搜索**

- [ ] **Step 4: 本地 UI 走查**（开关、标签区间、预览、保存错误提示）

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(frontend): add origin error page settings under websites"
```

---

### Task 5: 文档、changelog、收尾门禁

**Files:**
- Modify: `docs/reference/configuration.md` — 三 key 说明  
- Modify: `docs/changelog/index.md` `[unreleased]` 改进：用户可读中文  
- Modify: `docs/plan/index.md` — 本计划列入进行中  

- [ ] **Step 1: 写配置说明与 changelog**

示例 changelog：

```markdown
- 新增全局源站错误页：可在「网站管理 → 错误页」配置触发状态码（支持 500-599 区间与单码）与自定义 HTML；默认 Cloudflare 风格页面并保持真实 HTTP 状态码，关闭后恢复透传。
```

- [ ] **Step 2: `make code-check` 与前端 format**

- [ ] **Step 3: 手动验收清单（对照设计 §7.2）**

1. 默认启用 + 源站宕机 → 错误页 + 真实 status  
2. 源站 503 → 替换  
3. 仅 `522` → 其它 5xx 透传  
4. 关闭并发布 → 透传  
5. 自定义 `{{status}}`/`{{host}}`  
6. Pages 不受影响  

- [ ] **Step 4: Final commit**

```bash
git commit -m "docs: origin error page configuration and changelog"
```

---

## Spec coverage checklist

| Spec 要求 | Task |
|-----------|------|
| 全局开关 | 3, 4 |
| 默认 `500-599` / 单码+区间 | 1, 3, 4 |
| 状态码透传 + `{{status}}` | 2 |
| 默认 CF 风格 / 自定义 HTML | 2, 4 |
| Option + 配置版本 | 3 |
| 仅反代 | 2 |
| 侧栏网站管理 | 4 |
| Tags Input | 4 |
| 256KB 限制 | 3 |
| 测试与 changelog | 1–5 |

## Placeholder scan

无 TBD；Agent 路径替换若与现网 placeholder 机制不一致，在 Task 2 Step 3 内对齐现有 `CertDirPlaceholder` 模式，不另开悬空任务。
