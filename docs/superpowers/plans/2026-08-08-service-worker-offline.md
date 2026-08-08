# Service Worker 离线兜底 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给平台所有启用 HTTPS 的网站（反代 + Pages）下发 Service Worker 离线兜底：域名被墙后浏览器从缓存吐出"联系站长"页，避免用户流失。全平台一键批量下发。

**Architecture:** 全局 Option（SystemConfig / OpenRestyConfig snapshot）驱动，与现有 origin error page 完全同模式。渲染层在 HTTPS server 块注入 SW 静态 location + 首页挑战拦截（真实浏览器 UA 且无 cookie 时返回含 `register('/sw.js')` 的挑战页），通过 SupportFile 下发 sw.js / offline.html，Agent 替换占位符落盘。前端「响应页面」模块两个 tab：错误页 / 联系页。

**Tech Stack:** Go 1.25+、Gin、GORM、PostgreSQL/SQLite goose 迁移、OpenResty/Lua、Next.js、TypeScript、shadcn/ui、TanStack Query。

## Global Constraints

- 遵循 AGENTS.md 分层：`apps → repository → model`，禁止 `model → repository`。
- API 错误用 `response.Abort*`；Handler 不直接 `c.JSON`。
- 渲染改动后：`make swagger`（本功能无新 API，跳过）；开发完成：`make code-check`；提交前：`make format`。
- 代码/配置变更写入 `docs/changelog/index.md` 的 `[Unreleased]`（中文，用户可读）。
- 配置 key 命名：小写 snake_case。测试临时目录只用 `t.TempDir()`。
- 前端：`variant` + CSS 变量，业务 `className` 不硬编码颜色；根容器 `w-full`，外层 `py-6 px-1`；标题行 `flex items-center gap-2`。
- 配置文件路径占位符统一追加到 `pkg/render/openresty/types.go` 的 const 块。
- 迁移：PostgreSQL 与 SQLite 各一份 goose SQL（`goose/postgres/`、`goose/sqlite/`），见 `database-migration` skill。

---

### Task 1: 后端配置 key 与 Option 校验

**Files:**
- Modify: `internal/model/system_configs.go:114-117`
- Modify: `internal/apps/openflare/option/openresty_validators.go:19-65`
- Modify: `internal/apps/openflare/option/openresty_validators.go:69-79`

**Interfaces:**
- Produces: 常量 `model.ConfigKeySWOfflineEnabled`, `model.ConfigKeySWOfflineHTML`; 校验函数 `validateSWOfflineHTML`。

- [ ] **Step 1: 在 `system_configs.go` 追加 key 常量**

在 `ConfigKeyOriginErrorPageGetOnly`（第 117 行）后追加：

```go
	ConfigKeySWOfflineEnabled = "sw_offline_enabled" // 是否启用 Service Worker 离线兜底
	ConfigKeySWOfflineHTML    = "sw_offline_html"    // 离线联系页自定义 HTML（空则内置默认）
```

- [ ] **Step 2: 注册 validator**

在 `openRestyOptionValidators` map（`openresty_validators.go` 第 61-64 行）后追加：

```go
	model.ConfigKeySWOfflineEnabled: validateBooleanOption,
	model.ConfigKeySWOfflineHTML:    validateSWOfflineHTML,
```

- [ ] **Step 3: 在 `validateOpenRestyOption` 增加 HTML 字节数特殊处理**

在第 69-79 行函数内，`if key == model.ConfigKeyOriginErrorPageHTML` 分支改为同时覆盖 SW HTML：

```go
	if key == model.ConfigKeyOriginErrorPageHTML || key == model.ConfigKeySWOfflineHTML {
		return validateOriginErrorPageHTML(key, value)
	}
```

`validateOriginErrorPageHTML` 逻辑（非空、≤256 KiB）对两个 HTML 复用，无需新函数。

- [ ] **Step 4: 运行测试**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad && go build ./... && go test ./internal/apps/openflare/option/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/model/system_configs.go internal/apps/openflare/option/openresty_validators.go
git commit -m "feat(option): add sw offline config keys and validation"
```

---

### Task 2: goose 迁移（PostgreSQL + SQLite）Seed 全局 Option

**Files:**
- Create: `internal/infra/persistence/migrator/goose/postgres/<YYYYMMDD>NNN_add_sw_offline_options.sql`
- Create: `internal/infra/persistence/migrator/goose/sqlite/<YYYYMMDD>NNN_add_sw_offline_options.sql`

**Interfaces:**
- Consumes: Task 1 key 常量。
- Produces: 数据库 seed 的 `sw_offline_enabled` / `sw_offline_html` 两行 `w_system_configs`。

- [ ] **Step 1: 确认迁移序号**

Run: `ls /Users/ryan/conductor/workspaces/OpenFlare/islamabad/internal/infra/persistence/migrator/goose/postgres/ | tail -3`
取最新序号 +1（如 `202608080001`）。

- [ ] **Step 2: 创建 postgres 迁移**

创建 `goose/postgres/202608080001_add_sw_offline_options.sql`：

```sql
-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('sw_offline_enabled', 'false', 'business', 0, '是否启用 Service Worker 离线兜底', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('sw_offline_html', '', 'business', 0, '离线联系页自定义 HTML，空则使用内置默认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'sw_offline_enabled',
  'sw_offline_html'
);
```

- [ ] **Step 3: 创建 sqlite 迁移**

创建 `goose/sqlite/202608080001_add_sw_offline_options.sql`（内容与 postgres 相同）。

- [ ] **Step 4: 运行迁移测试**

Run: `go test ./internal/infra/persistence/migrator/...`
Expected: PASS（数据库迁移冒烟通过）

- [ ] **Step 5: 提交**

```bash
git add internal/infra/persistence/migrator/goose/postgres/202608080001_add_sw_offline_options.sql internal/infra/persistence/migrator/goose/sqlite/202608080001_add_sw_offline_options.sql
git commit -m "feat(db): seed sw offline options"
```

---

### Task 3: ConfigSnapshot 渲染类型字段

**Files:**
- Modify: `pkg/render/openresty/types.go:20-26`
- Modify: `pkg/render/openresty/types.go:318-323`（`ConfigSnapshot` 结构体）

**Interfaces:**
- Produces: `ConfigSnapshot.SWOfflineEnabled bool`、`ConfigSnapshot.SWOfflineHTML string`；常量 `SWDirPlaceholder`。

- [ ] **Step 1: 追加占位符常量**

在 `types.go` 占位符 const 块（第 23 行 `ErrorPageTmplPlaceholder` 后）追加：

```go
	SWDirPlaceholder = "__OPENFLARE_SW_DIR__"
```

- [ ] **Step 2: 追加 ConfigSnapshot 字段**

在 `ConfigSnapshot` 末尾（`OriginErrorPageGetOnly` 后）追加：

```go
	// SWOfflineEnabled enables the Service Worker offline fallback for HTTPS routes.
	SWOfflineEnabled bool `json:"sw_offline_enabled,omitempty"`
	// SWOfflineHTML is the contact-page HTML served offline; empty uses the built-in default.
	SWOfflineHTML string `json:"sw_offline_html,omitempty"`
```

- [ ] **Step 3: 提交**

```bash
git add pkg/render/openresty/types.go
git commit -m "feat(openresty): add sw offline ConfigSnapshot fields and placeholder"
```

---

### Task 4: 渲染层 SW 资源与挑战拦截

**Files:**
- Create: `pkg/render/openresty/service_worker.go`
- Create: `pkg/render/openresty/service_worker_test.go`
- Modify: `pkg/render/openresty/render.go:37-59`（`Render` 追加 support files）
- Modify: `pkg/render/openresty/render.go:90-115`（`RenderRouteConfig` 注入挑战）

**Interfaces:**
- Consumes: `ConfigSnapshot.SWOfflineEnabled` / `.SWOfflineHTML`；`SWDirPlaceholder`。
- Produces: `DefaultSWOfflineHTML string`、`EffectiveSWOfflineHTML(cfg ConfigSnapshot) string`、`ServiceWorkerSupportFiles(cfg ConfigSnapshot) []SupportFile`、`renderServiceWorkerChallenger(cfg ConfigSnapshot) string`。

- [ ] **Step 1: 写失败测试**

创建 `service_worker_test.go`，断言：
1. `EffectiveSWOfflineHTML`：HTML 为空返回内置默认；非空返回自定义。
2. `ServiceWorkerSupportFiles`：仅当 `SWOfflineEnabled` 时返回 `sw/sw.js` 与 `sw/offline.html` 两个文件；未启用返回 nil。
3. `renderServiceWorkerChallenger`：启用且含 sw.js location、offline location、挑战 location；未启用返回空串。

```go
package openresty

import (
	"strings"
	"testing"
)

func TestEffectiveSWOfflineHTML(t *testing.T) {
	if got := EffectiveSWOfflineHTML(ConfigSnapshot{}); got != DefaultSWOfflineHTML {
		t.Fatalf("default mismatch")
	}
	custom := "<html>custom</html>"
	if got := EffectiveSWOfflineHTML(ConfigSnapshot{SWOfflineHTML: custom}); got != custom {
		t.Fatalf("custom mismatch")
	}
}

func TestServiceWorkerSupportFiles(t *testing.T) {
	disabled := ServiceWorkerSupportFiles(ConfigSnapshot{})
	if disabled != nil {
		t.Fatalf("expected nil when disabled, got %v", disabled)
	}
	enabled := ServiceWorkerSupportFiles(ConfigSnapshot{SWOfflineEnabled: true})
	if len(enabled) != 2 {
		t.Fatalf("expected 2 support files, got %d", len(enabled))
	}
	paths := map[string]string{}
	for _, f := range enabled {
		paths[f.Path] = f.Content
	}
	if _, ok := paths["sw/sw.js"]; !ok {
		t.Fatalf("missing sw/sw.js")
	}
	if _, ok := paths["sw/offline.html"]; !ok {
		t.Fatalf("missing sw/offline.html")
	}
}

func TestRenderServiceWorkerChallenger(t *testing.T) {
	if got := renderServiceWorkerChallenger(ConfigSnapshot{}); got != "" {
		t.Fatalf("expected empty when disabled")
	}
	got := renderServiceWorkerChallenger(ConfigSnapshot{SWOfflineEnabled: true})
	for _, want := range []string{"location = /sw.js", "location = /offline.html", "sw.runtime", "content_by_lua"} {
		if !strings.Contains(got, want) {
			t.Fatalf("challenger missing %q", want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./pkg/render/openresty/ -run 'TestEffectiveSWOfflineHTML|TestServiceWorkerSupportFiles|TestRenderServiceWorkerChallenger'`
Expected: FAIL（函数未定义）

- [ ] **Step 3: 实现 `service_worker.go`**

```go
package openresty

import (
	"strings"
)

const (
	SWJSLocation      = "location = /sw.js"
	SWOfflineLocation = "location = /offline.html"
	SWChallengeLua    = "sw/challenge.lua"
	SWRuntimeLua      = "sw/runtime.lua"
	swDirPrefix       = "sw/"
)

// DefaultSWOfflineHTML is the built-in contact page shown when the domain is blocked.
const DefaultSWOfflineHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>网站暂时无法访问 | 联系站长</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #ffffff; color: #333333; height: 100vh; display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center; padding: 48px 24px; }
  h1 { font-size: 28px; font-weight: 700; margin-bottom: 16px; }
  p { font-size: 16px; line-height: 1.7; color: #666666; max-width: 520px; }
</style>
</head>
<body>
<h1>网站暂时无法访问</h1>
<p>当前域名暂时无法从网络访问。请通过其他方式联系网站管理员获取最新访问入口。</p>
</body>
</html>
`

// EffectiveSWOfflineHTML returns custom HTML when set, otherwise the built-in default.
func EffectiveSWOfflineHTML(cfg ConfigSnapshot) string {
	if strings.TrimSpace(cfg.SWOfflineHTML) == "" {
		return DefaultSWOfflineHTML
	}
	return cfg.SWOfflineHTML
}

// ServiceWorkerSupportFiles returns the sw.js script and offline contact page.
func ServiceWorkerSupportFiles(cfg ConfigSnapshot) []SupportFile {
	if !cfg.SWOfflineEnabled {
		return nil
	}
	return []SupportFile{
		{Path: swDirPrefix + "sw.js", Content: defaultSWJS()},
		{Path: swDirPrefix + "offline.html", Content: EffectiveSWOfflineHTML(cfg)},
	}
}

func defaultSWJS() string {
	return `var CACHE = "openflare-offline-v1";
var OFFLINE = "/offline.html";
self.addEventListener("install", function (e) {
  e.waitUntil(caches.open(CACHE).then(function (c) { return c.addAll([OFFLINE]); }));
  self.skipWaiting();
});
self.addEventListener("activate", function (e) {
  e.waitUntil(caches.keys().then(function (keys) {
    return Promise.all(keys.filter(function (k) { return k !== CACHE; }).map(function (k) { return caches.delete(k); }));
  }));
  self.clients.claim();
});
self.addEventListener("fetch", function (e) {
  if (e.request.method !== "GET") { return; }
  e.respondWith(
    fetch(e.request).catch(function () {
      return caches.match(e.request).then(function (r) { return r || caches.match(OFFLINE); });
    })
  );
});
`
}

// renderServiceWorkerChallenger emits SW static locations and the homepage
// challenge intercept for HTTPS server blocks.
func renderServiceWorkerChallenger(cfg ConfigSnapshot) string {
	if !cfg.SWOfflineEnabled {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("\n    location = /sw.js {\n")
	builder.WriteString("        alias " + SWDirPlaceholder + "/sw.js;\n")
	builder.WriteString("        default_type application/javascript;\n")
	builder.WriteString("        add_header Service-Worker-Allowed /;\n")
	builder.WriteString("        add_header Cache-Control \"no-cache\";\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    location = /offline.html {\n")
	builder.WriteString("        alias " + SWDirPlaceholder + "/offline.html;\n")
	builder.WriteString("        default_type text/html;\n")
	builder.WriteString("        add_header Cache-Control \"no-cache\";\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    location = /__openflare_sw_challenge {\n")
	builder.WriteString("        internal;\n")
	builder.WriteString("        content_by_lua_file " + SWDirPlaceholder + "/challenge.lua;\n")
	builder.WriteString("    }\n")
	return builder.String()
}
```

- [ ] **Step 4: 接入 `Render` 追加 support files**

在 `render.go` `Render` 函数内、`originErrorPageSupportFile` 追加之后追加：

```go
	if doc.OpenRestyConfig.SWOfflineEnabled {
		files = append(files, ServiceWorkerSupportFiles(doc.OpenRestyConfig)...)
	}
```

- [ ] **Step 5: 接入 `RenderRouteConfig` 注入挑战**

在 `RenderRouteConfig` 内 `renderProxyRoute` / `renderPagesRoute` 调用之前，将 SW 拦截接入 server 块。将 `renderAccessBlock(siteName, powEnabled)` 调用处扩展：新建 `renderServerAccess(siteName, powEnabled, cfg)` 封装，并在其中追加 SW 运行时检查。具体为在 `renderAccessBlock` 生成的 access 块内，追加对 `sw.runtime` 的调用。

简化实现：新增 `renderAccessBlockWithSW(siteName string, powEnabled bool, cfg ConfigSnapshot) string`，返回 `renderAccessBlock(siteName, powEnabled)` 与（当 `SWOfflineEnabled` 时）追加：

```
    access_by_lua_block {
        if not string.find(package.path, "__OPENFLARE_LUA_DIR__/?.lua", 1, true) then
            package.path = "__OPENFLARE_LUA_DIR__/?.lua;__OPENFLARE_LUA_DIR__/?/init.lua;" .. package.path
        end
        require("sw.runtime").check()
    }
```

然后将 `renderHTTPProxyServer`、`renderHTTPSServer`、`renderHTTPPagesServer`、`renderHTTPSPagesServer` 中 `renderAccessBlock(...)` 替换为 `renderAccessBlockWithSW(..., cfg)`，并在各自 server 块内追加 `renderServiceWorkerChallenger(cfg)` 输出。

**注意：** `renderAccessBlock` 在既有 powEnabled 分支已含 `access_by_lua_block`。为兼容，`renderAccessBlockWithSW` 在 powEnabled 分支内合并 SW check 到同一块；非 pow 分支额外追加一个块。本步以**仅新增 server 级 SW location + 独立 `access_by_lua_block`** 为最小实现；若 nginx 同 server 存在两个 `access_by_lua_block`，运行时只执行最后一个——**故实现必须合并**。请在实现时确认 `renderAccessBlock` 各分支，将 SW check 合并进唯一 access 块内，避免覆盖 WAF/PoW。

- [ ] **Step 6: 运行测试**

Run: `go test ./pkg/render/openresty/...`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add pkg/render/openresty/service_worker.go pkg/render/openresty/service_worker_test.go pkg/render/openresty/render.go
git commit -m "feat(openresty): render sw offline assets and challenge intercept"
```

---

### Task 5: config_version snapshot 接入全局 Option

**Files:**
- Modify: `internal/apps/openflare/config_version/snapshot.go:143-147`（`openRestyConfigSnapshot` 字段）
- Modify: `internal/apps/openflare/config_version/snapshot.go:559-563`（`buildOpenRestyConfigSnapshot` 读取）
- Modify: `internal/apps/openflare/config_version/logics.go:537-541`（diff 追加）
- Modify: `internal/apps/openflare/config_version/logics.go:604-608`（option keys 追加）

**Interfaces:**
- Consumes: `model.ConfigKeySWOfflineEnabled` / `.SWOfflineHTML`。
- Produces: snapshot JSON 内 `sw_offline_enabled` / `sw_offline_html` 字段，触发 checksum 变化。

- [ ] **Step 1: snapshot 结构体追加字段**

在 `openRestyConfigSnapshot`（`snapshot.go:143-147`，`OriginErrorPageGetOnly` 后）追加：

```go
	SWOfflineEnabled bool   `json:"sw_offline_enabled,omitempty"`
	SWOfflineHTML    string `json:"sw_offline_html,omitempty"`
```

- [ ] **Step 2: build 读取配置**

在 `buildOpenRestyConfigSnapshot`（`snapshot.go:559-563`，`OriginErrorPageGetOnly` 赋值后）追加：

```go
		SWOfflineEnabled: getBoolConfig(model.ConfigKeySWOfflineEnabled, false),
		SWOfflineHTML:    getStringConfig(model.ConfigKeySWOfflineHTML, ""),
```

- [ ] **Step 3: diff 追加**

在 `diffOpenRestyOptionDetails`（`logics.go:540` 后）追加：

```go
	appendIfChanged("SWOfflineEnabled", fmt.Sprintf("%t", left.SWOfflineEnabled), fmt.Sprintf("%t", right.SWOfflineEnabled))
	appendIfChanged("SWOfflineHTML", left.SWOfflineHTML, right.SWOfflineHTML)
```

- [ ] **Step 4: option keys 追加**

在 `openRestyOptionKeys()`（`logics.go:607` 后）追加：

```go
		"SWOfflineEnabled",
		"SWOfflineHTML",
```

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/apps/openflare/config_version/...`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/apps/openflare/config_version/snapshot.go internal/apps/openflare/config_version/logics.go
git commit -m "feat(config): wire sw offline options into config snapshot"
```

---

### Task 6: Agent 侧 SW Lua 资源与占位符替换

**Files:**
- Create: `internal/apps/agent/nginx/sw_assets.go`
- Modify: `internal/apps/agent/nginx/manager.go:393-410`（`EnsureLuaAssets` 追加 SW Lua）
- Modify: `internal/apps/agent/nginx/manager.go:526-528`（checksum 归一化 SW 路径）
- Modify: `internal/apps/agent/nginx/manager.go:1381-1383`（renderRouteConfig 替换 SW 占位符）

**Interfaces:**
- Consumes: `openrestyrender.SWDirPlaceholder`、`openrestyrender.SWChallengeLua`、`openrestyrender.SWRuntimeLua`。
- Produces: `ManagedSWLuaFiles() []protocol.SupportFile`（`sw/runtime.lua`、`sw/challenge.lua`）。

- [ ] **Step 1: 创建 `sw_assets.go`**

```go
package nginx

import (
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

const openRestySWRuntimeLua = `local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/sw/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

local function is_real_browser(ua)
    if not ua or ua == "" then return false end
    -- Chrome/Edge/CentOS-style: "Chrome/120"
    if string.find(ua, "Chrome/%d", 1, true) then return true end
    -- Firefox: "Firefox/120"
    if string.find(ua, "Firefox/%d", 1, true) then return true end
    -- Safari (non-Chrome, e.g. "Version/17.0 Safari")
    if not string.find(ua, "Chrome", 1, true) and string.find(ua, "Safari", 1, true) then return true end
    return false
end

local function pass_through()
    return true
end

function _M_check()
    local ua = ngx.var.http_user_agent or ""
    if not is_real_browser(ua) then return pass_through() end

    local uri = ngx.var.uri or ""
    if uri ~= "/" then return pass_through() end

    local cookie = ngx.var["cookie___openflare_sw"]
    if cookie and cookie ~= "" then return pass_through() end

    -- intercept: internal redirect to challenge page, which registers SW + sets cookie
    local redir = ngx.var.scheme .. "://" .. ngx.var.host .. uri .. (ngx.var.args and ("?" .. ngx.var.args) or "")
    ngx.req.set_uri_args({ redir = redir })
    return ngx.exec("/__openflare_sw_challenge")
end
`

const openRestySWChallengeLua = `local args = ngx.req.get_uri_args()
local redir = args["redir"] or "/"
ngx.header["Set-Cookie"] = "__openflare_sw=1; Path=/; Max-Age=31536000"
ngx.header.content_type = "text/html; charset=utf-8"
ngx.say([[<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>加载中...</title>
<script>
if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").then(function () {
    location.replace("]] .. redir .. [[");
  }).catch(function () {
    location.replace("]] .. redir .. [[");
  });
} else {
  location.replace("]] .. redir .. [[");
}
</script>
</head>
<body>正在加载...</body>
</html>]])
`

// ManagedSWLuaFiles returns embedded Lua assets for the SW offline challenge.
func ManagedSWLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "sw/runtime.lua", Content: openRestySWRuntimeLua},
		{Path: "sw/challenge.lua", Content: openRestySWChallengeLua},
	}
}
```

- [ ] **Step 2: `EnsureLuaAssets` 追加 SW Lua**

在 `manager.go:403`（`allSupportFiles` 组装处）追加：

```go
	allSupportFiles = append(allSupportFiles, ManagedSWLuaFiles()...)
```

- [ ] **Step 3: checksum 归一化 SW 路径**

在 `manager.go:526-528`（error page 路径归一化后）追加：

```go
		swDir := filepath.ToSlash(filepath.Join(m.NginxCertDir, "sw"))
		normalizedRoute = strings.ReplaceAll(normalizedRoute, swDir, openrestyrender.SWDirPlaceholder)
```

- [ ] **Step 4: `renderRouteConfig` 替换 SW 占位符**

在 `manager.go:1381-1383`（error page 替换后）追加：

```go
		swDir := filepath.ToSlash(filepath.Join(m.NginxCertDir, "sw"))
		rendered = strings.ReplaceAll(rendered, openrestyrender.SWDirPlaceholder, swDir)
```

- [ ] **Step 5: 确认 SW 文件落盘**

`renderServiceWorkerChallenger` 中 `alias __OPENFLARE_SW_DIR__/sw.js` 与 `/offline.html` 引用 support files `sw/sw.js`、`sw/offline.html`。这些文件经 Task 4 作为普通 support file 由 `writeManagedCertFiles` 写入 `<CertDir>/sw/`（路径含子目录）。验证 `certFileTargetPath` 支持子目录路径（读 `manager.go` 确认）。若不支持，需在 `writeManagedCertFiles` 中 `os.MkdirAll(filepath.Dir(targetPath))`。**实现时确认并补全目录创建。**

- [ ] **Step 6: 运行测试**

Run: `go build ./... && go test ./internal/apps/agent/nginx/...`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/apps/agent/nginx/sw_assets.go internal/apps/agent/nginx/manager.go
git commit -m "feat(agent): ship sw offline lua assets and placeholder substitution"
```

---

### Task 7: 前端「响应页面」模块（两个 tab）

**Files:**
- Create: `frontend/app/(main)/responses/page.tsx`
- Create: `frontend/app/(main)/responses/components/contact-page-tab.tsx`
- Create: `frontend/app/(main)/responses/components/shared.ts`
- Modify: `frontend/lib/navigation/openflare-nav.ts:63-67`
- Modify: `frontend/lib/navigation/openflare-nav.ts:123`

**Interfaces:**
- Consumes: `OptionService.list()` / `OptionService.updateBatch()`（已存在）。
- Produces: 联系页 tab 编辑 `sw_offline_enabled` / `sw_offline_html` 两个 option。

- [ ] **Step 1: 创建共享 helper `shared.ts`**

```ts
export const OPTIONS_QUERY_KEY = ['openflare', 'options'] as const;

export const KEY_SW_ENABLED = 'sw_offline_enabled';
export const KEY_SW_HTML = 'sw_offline_html';

export type ContactPageFields = {
  enabled: boolean;
  html: string;
};

export const defaultContactPageFields: ContactPageFields = {
  enabled: false,
  html: '',
};

export function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

export function mapOptionsToContactFields(
  optionMap: Record<string, string>,
): ContactPageFields {
  return {
    enabled: optionMap[KEY_SW_ENABLED] === 'true',
    html: optionMap[KEY_SW_HTML] ?? '',
  };
}

export async function invalidateResponseQueries(queryClient: {
  invalidateQueries: (opts: {
    queryKey: readonly unknown[];
  }) => Promise<unknown>;
}) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: OPTIONS_QUERY_KEY }),
    queryClient.invalidateQueries({
      queryKey: ['openflare', 'config-preview'],
    }),
    queryClient.invalidateQueries({
      queryKey: ['openflare', 'config-versions'],
    }),
  ]);
}
```

- [ ] **Step 2: 创建联系页 tab `contact-page-tab.tsx`**

参考 `error-pages/page.tsx` 交互：一个「启用」开关 + 一个 HTML 文本域 + 保存按钮。保存 `updateBatch([{key: KEY_SW_ENABLED,...},{key: KEY_SW_HTML,...}])`，成功后 `invalidateResponseQueries`。

```tsx
'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Save } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { OptionService } from '@/lib/services/openflare';

import {
  defaultContactPageFields,
  invalidateResponseQueries,
  KEY_SW_ENABLED,
  KEY_SW_HTML,
  mapOptionsToContactFields,
  optionsToMap,
  type ContactPageFields,
} from './shared';

export function ContactPageTab({ optionMap }: { optionMap: Record<string, string> }) {
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<ContactPageFields>(
    defaultContactPageFields,
  );

  useEffect(() => {
    setFields(mapOptionsToContactFields(optionMap));
  }, [optionMap]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      await OptionService.updateBatch([
        { key: KEY_SW_ENABLED, value: String(fields.enabled) },
        { key: KEY_SW_HTML, value: fields.html },
      ]);
    },
    onSuccess: async () => {
      toast.success('联系页已保存，请前往版本发布使配置生效');
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>离线兜底</CardTitle>
            <CardDescription>
              启用后给启用 HTTPS 的网站下发 Service Worker，域名被墙时浏览器从缓存展示此联系页。
            </CardDescription>
          </div>
          <Button
            size='sm'
            className='shrink-0'
            disabled={saveMutation.isPending}
            onClick={() => saveMutation.mutate()}
          >
            {saveMutation.isPending ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : (
              <Save className='size-3.5' />
            )}
            保存
          </Button>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-start justify-between gap-6'>
            <div className='space-y-1'>
              <Label className='text-sm font-medium'>启用 Service Worker 离线兜底</Label>
              <p className='text-sm text-muted-foreground'>
                仅对 HTTPS 网站生效；未启用的站点不受影响。
              </p>
            </div>
            <Switch
              checked={fields.enabled}
              onCheckedChange={(enabled) =>
                setFields((prev) => ({ ...prev, enabled }))
              }
              aria-label='启用离线兜底'
              className='mt-0.5 shrink-0'
            />
          </div>
          <div className='flex flex-col gap-3'>
            <Label htmlFor='sw-offline-html' className='text-sm font-medium'>
              离线联系页 HTML
            </Label>
            <p className='text-sm text-muted-foreground'>
              留空则使用内置默认模板。
            </p>
            <Textarea
              id='sw-offline-html'
              value={fields.html}
              onChange={(e) =>
                setFields((prev) => ({ ...prev, html: e.target.value }))
              }
              rows={12}
              className='font-mono'
              disabled={!fields.enabled}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
```

**注意：** 确认 `frontend/components/ui/` 存在 `textarea.tsx`（shadcn）。若无，用 `make sure` 或 `npx shadcn@latest add textarea` 添加。

- [ ] **Step 3: 创建页面容器 `responses/page.tsx`**

用 Tabs 组件组织「错误页」「联系页」两个 tab。错误页 tab 复用现有 `error-pages` 内容或重定向；联系页 tab 渲染 `ContactPageTab`。加载 `OptionService.list()` 传入 optionMap。

**实现提示：** 为避免重复，错误页 tab 的现有逻辑（`error-pages/page.tsx` 的 policy 卡片 + 预览卡）可先以 `redirect` 到 `/error-pages` 占位，或直接在容器内嵌两 tab。推荐：容器页 `responses/page.tsx` 读取 options，渲染 Tabs（错误页/联系页），错误页 tab 复用 `frontend/app/(main)/error-pages` 现有 UI（通过 import 其组件或在容器内重构）。**保守实现：** 容器页仅放两个 tab，错误页 tab 用 `<Link href='/error-pages'>` 或保留现有 `/error-pages` 路由，联系页 tab 显示新表单；导航入口改为「响应页面」指向 `/responses`。

- [ ] **Step 4: 更新导航**

`openflare-nav.ts` 第 63-67 行将「错误页」项改为「响应页面」：

```ts
    {
      title: '响应页面',
      url: '/responses',
      childUrls: ['/error-pages', '/responses/contact'],
    },
```

第 123 行 `openflareWebsiteSubNav` 中 `{ title: '错误页', url: '/error-pages' }` 改为 `{ title: '响应页面', url: '/responses' }`。

- [ ] **Step 5: 构建前端**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad/frontend && pnpm type-check`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add frontend/app/\(main\)/responses frontend/lib/navigation/openflare-nav.ts
git commit -m "feat(frontend): add response pages module with contact page tab"
```

---

### Task 8: Changelog 与收尾验证

**Files:**
- Modify: `docs/changelog/index.md`

**Interfaces:**
- Produces: `[Unreleased]` 下用户可读中文条目。

- [ ] **Step 1: 追加 changelog**

在 `docs/changelog/index.md` 的 `[Unreleased]` 下追加：

```markdown
### 新增

- 支持 Service Worker 离线兜底：为启用 HTTPS 的网站下发 Service Worker 并缓存离线联系页，域名无法访问时浏览器展示联系站长页面，减少用户流失。配置位于「响应页面」-「联系页」，可在版本发布中批量生效。
```

- [ ] **Step 2: 运行完整校验**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad && make code-check`
Expected: PASS（golangci-lint + 前端类型检查）

- [ ] **Step 3: 运行后端全量测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: 格式化**

Run: `cd /Users/ryan/conductor/workspaces/OpenFlare/islamabad && make format`
Expected: 无格式变更或已应用

- [ ] **Step 5: 提交**

```bash
git add docs/changelog/index.md
git commit -m "docs: sw offline fallback changelog"
```

---

## Self-Review

**Spec 覆盖检查：**
- 全局 Option（sw_offline_enabled / html）→ Task 1-3、5 ✓
- 渲染层 SW 静态 + 挑战拦截（反代 + Pages，HTTPS-only）→ Task 4 ✓
- SupportFile 下发 sw.js / offline.html，Agent 占位符替换 → Task 4、6 ✓
- UA 白名单（真实浏览器特征）→ Task 6 `is_real_browser` ✓
- Cookie 长过期 + 首次挑战页 → Task 6 ✓
- 前端「响应页面」两 tab → Task 7 ✓
- 迁移 seed → Task 2 ✓
- Changelog → Task 8 ✓

**占位符扫描：** 无 TBD/TODO。Task 4 Step 5 与 Task 7 Step 3 保留实现细节提示（非占位，是给定方向让执行者按实际代码确认），已在文中明确标注"实现时确认"。

**类型一致性：** `ConfigSnapshot.SWOfflineEnabled/HTML` 在 Task 3/4/5 一致；`SWDirPlaceholder` 在 Task 3/4/6 一致；`sw_offline_enabled/sw_offline_html` key 在 Task 1/2/5/7 一致；`renderServiceWorkerChallenger`/`ServiceWorkerSupportFiles`/`EffectiveSWOfflineHTML`/`ManagedSWLuaFiles` 签名跨 Task 一致。

**已知待确认项（执行时需按实际代码落地）：**
- Task 4：`renderAccessBlock` 的 access 块合并（避免 WAF/PoW 被覆盖）。
- Task 6：`certFileTargetPath` 是否支持子目录，落盘目录创建。
- Task 7：`textarea` 组件存在性；「响应页面」错误页 tab 与现有 `/error-pages` 路由的复用策略。
