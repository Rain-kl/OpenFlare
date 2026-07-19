# 边缘限流全局默认 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为边缘限流增加三项全局默认；站点 `0`/空继承默认、`-1` 显式关闭、`>0` 覆盖；在 `RenderRouteConfig` 唯一合并。

**Architecture:** 全局默认存 `system_configs`，进入 `openresty_config` 快照；站点字段语义变更后仍原样入库与快照；`pkg/render/openresty.RenderRouteConfig` 用 `doc.OpenRestyConfig` 与 route 字段合并后输出 location 指令。UI：安全性下新页「限流」+ 站点限流文案更新。

**Tech Stack:** Go、goose SQL、Option API、`pkg/render/openresty`、Next.js、OptionService

**Spec:** [docs/superpowers/specs/2026-07-19-http-default-rate-limit-design.md](../specs/2026-07-19-http-default-rate-limit-design.md)

## Global Constraints

- 合并**只**在 `RenderRouteConfig`；快照保留站点原始值（含 `0`/`-1`）
- 不引入 `limit_req`；不在 `http {}` 写默认 `limit_conn`/`limit_rate`
- 全局默认初始 `0`/空 → 存量行为不变
- 完成后 `make code-check`；改前端后 `make prettier`；中文 changelog；不写英文文档
- 所有 HTTP 路由仍只在 `internal/router/router.go` 委派（本功能复用 Option API，无需新业务路由）

## File map

| 文件 | 职责 |
|------|------|
| `internal/model/system_configs.go` | 三个 ConfigKey 常量 |
| `internal/db/migrator/goose/{postgres,sqlite}/202607190001_add_openresty_default_rate_limits.sql` | seed 默认值 |
| `internal/apps/openflare/option/openresty_validators.go` + `validate.go` | 全局默认校验 |
| `internal/apps/openflare/config_version/snapshot.go` | 快照字段 + 读取 |
| `internal/apps/openflare/config_version/logics.go` | option diff keys |
| `pkg/render/openresty/types.go` | `ConfigSnapshot` 三字段 |
| `pkg/render/openresty/render.go` | `mergeRouteLimit*` + 调用点 |
| `pkg/render/openresty/render_test.go` | 合并渲染单测 |
| `internal/apps/openflare/proxy_route/helpers.go` | 站点 normalize 允许 -1 |
| `frontend/lib/navigation/openflare-nav.ts` | 安全性子菜单 |
| `frontend/app/(main)/rate-limits/page.tsx` | 全局限流设置页 |
| `frontend/app/(main)/proxy-routes/.../limits-section.tsx` + helpers | 站点语义 UI |
| `frontend/lib/utils/search-data.ts` | 搜索入口 |
| `docs/reference/configuration.md` | 配置键说明 |
| `docs/changelog/index.md` | Unreleased |
| `docs/plan/index.md` | 进行中计划索引 |

---

### Task 1: Render 合并（TDD 核心）

**Files:**
- Modify: `pkg/render/openresty/types.go` (`ConfigSnapshot`)
- Modify: `pkg/render/openresty/render.go`
- Test: `pkg/render/openresty/render_test.go`

**Interfaces:**
- Produces: `ConfigSnapshot` 字段 `DefaultLimitConnPerServer int`, `DefaultLimitConnPerIP int`, `DefaultLimitRate string`（json: `default_limit_conn_per_server` 等）
- Produces: `mergeRouteLimitConfig(route Route, cfg ConfigSnapshot) routeLimitConfig`
- Produces: `mergeLimitConn(route, def int) int`, `mergeLimitRate(route, def string) string`

- [ ] **Step 1: 写失败单测**

在 `render_test.go` 末尾追加：

```go
func TestMergeRouteLimitConfig(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		route  Route
		cfg    ConfigSnapshot
		want   routeLimitConfig
	}{
		{
			name:  "both zero off",
			route: Route{},
			cfg:   ConfigSnapshot{},
			want:  routeLimitConfig{},
		},
		{
			name:  "inherit all defaults",
			route: Route{},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
			},
			want: routeLimitConfig{LimitConnPerServer: 100, LimitConnPerIP: 10, LimitRate: "512k"},
		},
		{
			name:  "explicit off ignores default",
			route: Route{LimitConnPerServer: -1, LimitConnPerIP: -1, LimitRate: "-1"},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
			},
			want: routeLimitConfig{},
		},
		{
			name:  "route overrides default",
			route: Route{LimitConnPerServer: 50, LimitConnPerIP: 5, LimitRate: "1m"},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "512k",
			},
			want: routeLimitConfig{LimitConnPerServer: 50, LimitConnPerIP: 5, LimitRate: "1m"},
		},
		{
			name:  "partial inherit",
			route: Route{LimitConnPerServer: 0, LimitConnPerIP: -1, LimitRate: ""},
			cfg: ConfigSnapshot{
				DefaultLimitConnPerServer: 100,
				DefaultLimitConnPerIP:     10,
				DefaultLimitRate:          "256k",
			},
			want: routeLimitConfig{LimitConnPerServer: 100, LimitConnPerIP: 0, LimitRate: "256k"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergeRouteLimitConfig(tc.route, tc.cfg)
			if got != tc.want {
				t.Fatalf("mergeRouteLimitConfig() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestRenderRouteConfigAppliesDefaultLimits(t *testing.T) {
	doc := Document{
		Routes: []Route{{
			SiteName: "example.com",
			Domains:  []string{"example.com"},
			Enabled:  true,
			OriginURL: "http://127.0.0.1:8080",
			Upstreams: []string{"http://127.0.0.1:8080"},
		}},
		OpenRestyConfig: ConfigSnapshot{
			DefaultLimitConnPerServer: 120,
			DefaultLimitConnPerIP:     12,
			DefaultLimitRate:          "512k",
		},
	}
	rendered, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	for _, want := range []string{
		"limit_conn openflare_conn_per_server 120;",
		"limit_conn openflare_conn_per_ip 12;",
		"limit_rate 512k;",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in route config, got:\n%s", want, rendered)
		}
	}
}

func TestRenderRouteConfigExplicitOffSkipsDefaultLimits(t *testing.T) {
	doc := Document{
		Routes: []Route{{
			SiteName:           "example.com",
			Domains:            []string{"example.com"},
			Enabled:            true,
			OriginURL:          "http://127.0.0.1:8080",
			Upstreams:          []string{"http://127.0.0.1:8080"},
			LimitConnPerServer: -1,
			LimitConnPerIP:     -1,
			LimitRate:          "-1",
		}},
		OpenRestyConfig: ConfigSnapshot{
			DefaultLimitConnPerServer: 120,
			DefaultLimitConnPerIP:     12,
			DefaultLimitRate:          "512k",
		},
	}
	rendered, err := RenderRouteConfig(doc, nil)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	if strings.Contains(rendered, "limit_conn") || strings.Contains(rendered, "limit_rate") {
		t.Fatalf("expected no limit directives, got:\n%s", rendered)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

```bash
go test ./pkg/render/openresty/ -run 'TestMergeRouteLimitConfig|TestRenderRouteConfigAppliesDefaultLimits|TestRenderRouteConfigExplicitOffSkipsDefaultLimits' -count=1
```

Expected: FAIL（`mergeRouteLimitConfig` undefined 或行为不符）

- [ ] **Step 3: 实现 types + merge + 调用**

`ConfigSnapshot` 增加：

```go
DefaultLimitConnPerServer int    `json:"default_limit_conn_per_server,omitempty"`
DefaultLimitConnPerIP     int    `json:"default_limit_conn_per_ip,omitempty"`
DefaultLimitRate          string `json:"default_limit_rate,omitempty"`
```

`render.go` 中 `RenderRouteConfig` 将：

```go
limitConfig := routeLimitConfig{LimitConnPerServer: route.LimitConnPerServer, LimitConnPerIP: route.LimitConnPerIP, LimitRate: route.LimitRate}
```

改为：

```go
limitConfig := mergeRouteLimitConfig(route, doc.OpenRestyConfig)
```

并新增：

```go
func mergeRouteLimitConfig(route Route, cfg ConfigSnapshot) routeLimitConfig {
	return routeLimitConfig{
		LimitConnPerServer: mergeLimitConn(route.LimitConnPerServer, cfg.DefaultLimitConnPerServer),
		LimitConnPerIP:     mergeLimitConn(route.LimitConnPerIP, cfg.DefaultLimitConnPerIP),
		LimitRate:          mergeLimitRate(route.LimitRate, cfg.DefaultLimitRate),
	}
}

func mergeLimitConn(route, def int) int {
	if route == -1 {
		return 0
	}
	if route > 0 {
		return route
	}
	if def > 0 {
		return def
	}
	return 0
}

func mergeLimitRate(route, def string) string {
	r := strings.ToLower(strings.TrimSpace(route))
	if r == "-1" {
		return ""
	}
	if r != "" && r != "0" {
		return r
	}
	d := strings.ToLower(strings.TrimSpace(def))
	if d != "" && d != "0" {
		return d
	}
	return ""
}
```

- [ ] **Step 4: 跑测通过**

```bash
go test ./pkg/render/openresty/ -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/render/openresty/types.go pkg/render/openresty/render.go pkg/render/openresty/render_test.go
git commit -m "feat(openresty): merge global default limits at route render"
```

---

### Task 2: 配置键、迁移、校验、快照

**Files:**
- Modify: `internal/model/system_configs.go`
- Create: `internal/db/migrator/goose/postgres/202607190001_add_openresty_default_rate_limits.sql`
- Create: `internal/db/migrator/goose/sqlite/202607190001_add_openresty_default_rate_limits.sql`
- Modify: `internal/apps/openflare/option/validate.go`
- Modify: `internal/apps/openflare/option/openresty_validators.go`
- Modify: `internal/apps/openflare/config_version/snapshot.go`
- Modify: `internal/apps/openflare/config_version/logics.go`

**Interfaces:**
- Consumes: Task 1 的 `ConfigSnapshot` JSON 字段名
- Produces: `ConfigKeyOpenRestyDefaultLimitConnPerServer` 等三常量；snapshot 填充；diff 可见

- [ ] **Step 1: 常量**

在 `system_configs.go` OpenResty 段末尾（`MainConfigTemplate` 前或后）加入：

```go
ConfigKeyOpenRestyDefaultLimitConnPerServer = "openresty_default_limit_conn_per_server" // 默认站点并发连接
ConfigKeyOpenRestyDefaultLimitConnPerIP     = "openresty_default_limit_conn_per_ip"     // 默认单 IP 并发连接
ConfigKeyOpenRestyDefaultLimitRate          = "openresty_default_limit_rate"            // 默认单请求带宽
```

- [ ] **Step 2: goose 迁移（PG + SQLite 同内容）**

```sql
-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('openresty_default_limit_conn_per_server', '0', 'business', 0, '默认站点并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('openresty_default_limit_conn_per_ip', '0', 'business', 0, '默认单 IP 并发连接上限（0 关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('openresty_default_limit_rate', '', 'business', 0, '默认单请求带宽限速（空关闭）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
  'openresty_default_limit_conn_per_server',
  'openresty_default_limit_conn_per_ip',
  'openresty_default_limit_rate'
);
```

SQLite：若项目其它 seed 不用 `ON CONFLICT`，对照 `202607170001_add_pages_system_configs.sql` 的 sqlite  twin 写法保持一致（通常可同用 `ON CONFLICT (key) DO NOTHING`）。

- [ ] **Step 3: 校验器**

`validate.go` 增加：

```go
func validateNonNegativeIntegerOption(key, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue < 0 {
		return fmt.Errorf("%s 必须为大于等于 0 的整数", key)
	}
	return nil
}
```

`openresty_validators.go` 注册：

```go
model.ConfigKeyOpenRestyDefaultLimitConnPerServer: validateNonNegativeIntegerOption,
model.ConfigKeyOpenRestyDefaultLimitConnPerIP:     validateNonNegativeIntegerOption,
model.ConfigKeyOpenRestyDefaultLimitRate:          validateOpenRestyDefaultLimitRate,
```

```go
var openRestyDefaultLimitRatePattern = regexp.MustCompile(`^\d+[kKmM]?$`)

func validateOpenRestyDefaultLimitRate(key, trimmed string) error {
	if trimmed == "" || trimmed == "0" {
		return nil
	}
	if !openRestyDefaultLimitRatePattern.MatchString(strings.ToLower(trimmed)) {
		return fmt.Errorf("%s 格式不合法，请使用 512k、1m 或纯数字，空表示关闭", key)
	}
	return nil
}
```

- [ ] **Step 4: 快照读取（注意 0 合法）**

`openRestyConfigSnapshot` 与 `buildOpenRestyConfigSnapshot` 增加三字段。

**禁止**对这三项使用现有 `getIntConfig`（其 `val <= 0` 会把合法 `0` 与错误混在一起；虽 default=0 时偶然正确，但语义不清）。改为：

```go
getNonNegIntConfig := func(key string, defaultVal int) int {
	val, err := repository.GetIntByKey(ctx, key)
	if err != nil || val < 0 {
		return defaultVal
	}
	return val
}
```

```go
DefaultLimitConnPerServer: getNonNegIntConfig(model.ConfigKeyOpenRestyDefaultLimitConnPerServer, 0),
DefaultLimitConnPerIP:     getNonNegIntConfig(model.ConfigKeyOpenRestyDefaultLimitConnPerIP, 0),
DefaultLimitRate:          strings.ToLower(strings.TrimSpace(getStringConfig(model.ConfigKeyOpenRestyDefaultLimitRate, ""))),
```

若 `DefaultLimitRate == "0"`，规范化为 `""`。

确认 snapshot → render JSON 字段名与 `openrestyrender.ConfigSnapshot` 一致（`snapshotDocument` 序列化后由 `RenderJSON` 反序列化到 render types）。`openRestyConfigSnapshot` 的 json tag 必须与 `ConfigSnapshot` 对齐：

```go
DefaultLimitConnPerServer int    `json:"default_limit_conn_per_server,omitempty"`
DefaultLimitConnPerIP     int    `json:"default_limit_conn_per_ip,omitempty"`
DefaultLimitRate          string `json:"default_limit_rate,omitempty"`
```

- [ ] **Step 5: option diff**

在 `diffOpenRestyOptionDetails` 末尾：

```go
appendIfChanged("OpenRestyDefaultLimitConnPerServer", fmt.Sprintf("%d", left.DefaultLimitConnPerServer), fmt.Sprintf("%d", right.DefaultLimitConnPerServer))
appendIfChanged("OpenRestyDefaultLimitConnPerIP", fmt.Sprintf("%d", left.DefaultLimitConnPerIP), fmt.Sprintf("%d", right.DefaultLimitConnPerIP))
appendIfChanged("OpenRestyDefaultLimitRate", left.DefaultLimitRate, right.DefaultLimitRate)
```

`openRestyOptionKeys()` 同步追加这三 key 字符串。

- [ ] **Step 6: 编译/相关测试**

```bash
go test ./internal/apps/openflare/config_version/ ./internal/apps/openflare/option/ ./pkg/render/openresty/ -count=1
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/model/system_configs.go \
  internal/db/migrator/goose/postgres/202607190001_add_openresty_default_rate_limits.sql \
  internal/db/migrator/goose/sqlite/202607190001_add_openresty_default_rate_limits.sql \
  internal/apps/openflare/option/validate.go \
  internal/apps/openflare/option/openresty_validators.go \
  internal/apps/openflare/config_version/snapshot.go \
  internal/apps/openflare/config_version/logics.go
git commit -m "feat(config): add openresty default rate limit system options"
```

---

### Task 3: 站点 normalize 允许 -1

**Files:**
- Modify: `internal/apps/openflare/proxy_route/helpers.go`
- Modify: `internal/apps/openflare/proxy_route/errs.go`（如需更新文案）
- Test: 若无现成 helpers 测试文件则新建 `helpers_limit_test.go`

**Interfaces:**
- Produces: `normalizeProxyRouteLimitConnValue` 允许 `>= -1`；`normalizeProxyRouteLimitRate` 允许 `"-1"`

- [ ] **Step 1: 失败单测**

```go
func TestNormalizeProxyRouteLimitConnValue(t *testing.T) {
	t.Parallel()
	got, err := normalizeProxyRouteLimitConnValue(-1, "limit_conn_per_server")
	if err != nil || got != -1 {
		t.Fatalf("want -1, got %d err %v", got, err)
	}
	if _, err := normalizeProxyRouteLimitConnValue(-2, "limit_conn_per_server"); err == nil {
		t.Fatal("expected error for -2")
	}
}

func TestNormalizeProxyRouteLimitRate(t *testing.T) {
	t.Parallel()
	got, err := normalizeProxyRouteLimitRate("-1")
	if err != nil || got != "-1" {
		t.Fatalf("want -1, got %q err %v", got, err)
	}
	got, err = normalizeProxyRouteLimitRate("0")
	if err != nil || got != "" {
		t.Fatalf("want empty inherit, got %q err %v", got, err)
	}
}
```

- [ ] **Step 2: 实现**

```go
func normalizeProxyRouteLimitConnValue(value int, field string) (int, error) {
	if value < -1 {
		return 0, fmt.Errorf("%s must be greater than or equal to -1", field)
	}
	return value, nil
}

func normalizeProxyRouteLimitRate(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == "0" {
		return "", nil
	}
	if normalized == "-1" {
		return "-1", nil
	}
	if !proxyRouteLimitRatePattern.MatchString(normalized) {
		return "", errors.New(errProxyRouteLimitRate)
	}
	if strings.TrimRight(normalized, "km") == "" {
		return "", nil
	}
	return normalized, nil
}
```

可选：`errProxyRouteLimitRate` 文案追加「或 -1 表示关闭」。

- [ ] **Step 3: 测试**

```bash
go test ./internal/apps/openflare/proxy_route/ -count=1
```

- [ ] **Step 4: Commit**

```bash
git add internal/apps/openflare/proxy_route/
git commit -m "feat(proxy-route): allow -1 to disable rate limits"
```

---

### Task 4: 前端 — 安全性「限流」页 + 站点文案

**Files:**
- Modify: `frontend/lib/navigation/openflare-nav.ts`
- Create: `frontend/app/(main)/rate-limits/page.tsx`
- Modify: `frontend/app/(main)/proxy-routes/detail/components/limits-section.tsx`
- Modify: `frontend/app/(main)/proxy-routes/components/helpers.ts`
- Modify: `frontend/lib/utils/search-data.ts`

**Interfaces:**
- Consumes: Option keys 字面量 `openresty_default_limit_conn_per_server` 等
- Produces: `/rate-limits` 管理页；站点表单接受 `-1`

- [ ] **Step 1: 导航**

`openflareSecurityNavGroup.items`：

```ts
{ title: 'WAF', url: '/waf' },
{ title: 'IP 组', url: '/ip-groups' },
{ title: '限流', url: '/rate-limits' },
```

- [ ] **Step 2: 搜索**

`search-data.ts` 在 IP 组后增加：

```ts
{
  id: 'console-rate-limits',
  title: '限流',
  description: '配置边缘站点默认并发与带宽限流策略',
  url: '/rate-limits',
  category: 'page',
  keywords: ['限流', 'rate limit', 'limit_conn', 'limit_rate', '并发', '带宽'],
},
```

- [ ] **Step 3: 限流设置页**

新建 `frontend/app/(main)/rate-limits/page.tsx`，模式对齐 `performance/page.tsx`：

- `useAuth` 管理员校验
- `OptionService.list` / `updateBatch`
- 三字段表单 + 单卡片保存
- 标题：`Shield` 或 `Gauge` 图标 + `h1`「限流」
- 描述：空/0 表示默认关闭；修改后需在版本发布中生效
- keys：
  - `openresty_default_limit_conn_per_server`
  - `openresty_default_limit_conn_per_ip`
  - `openresty_default_limit_rate`
- conn：非负整数；rate：空或 `^\d+[kKmM]?$`
- 保存成功 toast + invalidate options / config-preview / config-versions
- 链到 `/config-versions`

页面骨架要点（完整实现时展开为完整组件，勿留半成品）：

```tsx
// 字段 state、OptionService.list map、updateBatch([{key,value},...])
// 文案：「0 或空表示默认关闭；站点未单独配置时继承此处设置。」
```

- [ ] **Step 4: 站点 limits-section**

1. schema：conn 允许空、`0`、`-1`、正整数：

```ts
if (!rawValue) continue;
if (!/^-1$|^\d+$/.test(rawValue)) {
  context.addIssue({ ..., message: '请输入 -1、0 或正整数' });
}
```

2. `validateLimitRate` / `normalizeLimitRate`：

```ts
export function validateLimitRate(value: string) {
  const normalized = value.trim();
  if (!normalized || normalized === '0' || normalized === '-1') {
    return null;
  }
  if (!limitRatePattern.test(normalized)) {
    return '限速格式不合法，请使用 512k、1m、纯数字，或 -1 关闭';
  }
  return null;
}

export function normalizeLimitRate(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized === '0') return '';
  return normalized; // 保留 -1
}
```

3. 表单展示：`-1` 需显示为 `'-1'`（注意 `route.limit_conn_per_server ? String : ''` 对 `-1` 已为 truthy；对 `0` 仍为空）

4. 提交：空 → `0`；`-1` → `-1`；正数 → 数字

5. 文案：

```
description='站点限流。空或 0 继承全局默认；-1 显式关闭；大于 0 为自定义。'
FormDescription 同步说明
```

6. 侧栏「流量限制」section description 可改为：`设置连接数和限速（可继承全局默认）。`

- [ ] **Step 5: prettier + 类型检查（按项目习惯）**

```bash
make prettier
# 若有前端 typecheck：
# cd frontend && pnpm exec tsc --noEmit
```

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/navigation/openflare-nav.ts \
  frontend/app/\(main\)/rate-limits/ \
  frontend/app/\(main\)/proxy-routes/detail/components/limits-section.tsx \
  frontend/app/\(main\)/proxy-routes/components/helpers.ts \
  frontend/lib/utils/search-data.ts
git commit -m "feat(frontend): add security rate-limits page and inherit UI"
```

---

### Task 5: 文档、索引、门禁

**Files:**
- Modify: `docs/reference/configuration.md`（OpenResty 配置表）
- Modify: `docs/changelog/index.md` `[unreleased]`
- Modify: `docs/plan/index.md`

- [ ] **Step 1: configuration.md**

在 `openresty_cache_use_stale` 与 `openresty_main_config_template` 之间插入：

```md
| `openresty_default_limit_conn_per_server` | `int` | 站点未配置时的默认并发连接上限；`0` 表示默认关闭 | `0` |
| `openresty_default_limit_conn_per_ip` | `int` | 站点未配置时的默认单 IP 并发上限；`0` 表示默认关闭 | `0` |
| `openresty_default_limit_rate` | `string` | 站点未配置时的默认单请求带宽（如 `512k`）；空表示默认关闭 | 空 |
```

- [ ] **Step 2: changelog**

`[unreleased]` 下：

```md
### 新增

- 安全性新增「限流」设置：可为边缘站点配置默认并发与带宽；站点未设置时继承，填 `-1` 可显式关闭。

### 改进

- 站点流量限制语义调整为空或 `0` 继承全局默认、`-1` 关闭、大于 `0` 自定义；修改全局默认后需发布配置版本生效。
```

- [ ] **Step 3: plan index**

`docs/plan/index.md` 进行中列表增加：

```md
* [边缘限流全局默认](../superpowers/plans/2026-07-19-http-default-rate-limit.md)：http/全局默认限流，站点 0 继承、-1 关闭。
```

- [ ] **Step 4: 全量门禁**

```bash
make code-check
make prettier
```

Expected: 通过；修复任何报错后再提交。

- [ ] **Step 5: Commit**

```bash
git add docs/reference/configuration.md docs/changelog/index.md docs/plan/index.md
git commit -m "docs: document default edge rate limits"
```

---

## Spec coverage checklist

| Spec 要求 | Task |
|-----------|------|
| 三项全局默认 | 2, 4 |
| 0/空继承、-1 关、>0 覆盖 | 1, 3, 4 |
| 仅 `RenderRouteConfig` 合并 | 1 |
| 快照保留原始站点值 | 2（不写回 route） |
| 安全性子页「限流」 | 4 |
| 初始 0/空兼容 | 2 seed |
| option diff / 发布 | 2 |
| 测试合并/normalize | 1, 3 |
| 中文文档/changelog | 5 |
| 非目标 limit_req / http 级指令 | 未做 |

## 手动验收

1. 迁移后三键存在且为 `0`/空
2. 安全性 → 限流 设置 `120` / `12` / `512k` 并保存
3. 版本发布预览：未配置站点的 location 出现对应 `limit_conn`/`limit_rate`
4. 站点将该项改为 `-1` 保存并发布：该维度指令消失
5. 站点改为 `50`：输出 50 而非全局值
