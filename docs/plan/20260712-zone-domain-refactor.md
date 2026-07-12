# Zone 与域名资源重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以稳定 ID 的 Zone 管理入口和正规化 Zone 域名替代 `managed_domains` 及反代路由中的域名/证书冗余字段，同时保持配置发布后的 OpenResty 行为不变。

**Architecture:** `of_zones` 管理可注册根域；`of_zone_domains` 是明确 FQDN、证书和反代路由之间的唯一关联来源。反代路由保留路由策略，配置快照在控制面联查 Zone 域名与证书后生成现有 OpenResty 配置格式。第一发布阶段保留旧列供可重复执行的历史数据导入读取；生产快照对比通过后才执行第二阶段清理。

**Tech Stack:** Go 1.25、Gin、GORM、goose（PostgreSQL/SQLite）、`golang.org/x/net/publicsuffix`、Next.js App Router、TypeScript、TanStack Query、shadcn/ui。

## Global Constraints

* Zone URL 必须为 `/websites/:zoneId`，不得使用域名作为路由参数。
* Zone 根域由 `publicsuffix.EffectiveTLDPlusOne` 解析；Zone 域名只接受明确 FQDN，拒绝 `*.`。
* TLS 证书可含通配符 SAN；证书只能由 `of_zone_domains.cert_id` 指定，`of_proxy_routes` 不再保存证书字段。
* `of_zone_domains.domain` 全局唯一；同一 Zone 域名至多绑定一条反代路由，路由可关联多个 Zone 的域名。
* 不建立物理数据库外键；所有关联列必须建立显式索引。
* 所有 HTTP 路由仅通过 `internal/router/v1/openflare/` 的管理端注册器委派；Handler 使用 `response.Abort*` 报错并补全 Swagger。
* 不新增 DNS 记录管理、边缘函数、预览子域或多租户能力。
* 每次 API 变更运行 `make swagger`；每个实现任务结束运行对应测试；完成前必须运行 `make code-check`。

---

## File Structure

| 路径 | 职责 |
| --- | --- |
| `internal/db/migrator/goose/{postgres,sqlite}/202607120001_create_zone_domain_tables.sql` | 第一阶段 Zone/ZoneDomain DDL 与索引。 |
| `internal/model/openflare_zone.go` | Zone、ZoneDomain 模型及数据访问。 |
| `internal/apps/openflare/zone/{logics.go,routers.go,errs.go,legacy_import.go}` | Zone CRUD、概览、输入验证和历史导入。 |
| `internal/cmd/migrate_zones.go` | 显式、可重复运行的历史 Zone 数据导入命令。 |
| `internal/router/v1/openflare/register_zone.go` | `/api/v1/d/zones` 路由注册。 |
| `internal/apps/openflare/proxy_route/*` | 以 `zone_domain_ids` 取代域名与证书输入。 |
| `internal/apps/openflare/config_version/*`、`pkg/render/openresty/*` | 快照与 OpenResty 渲染改为使用 Zone 域名。 |
| `frontend/lib/services/openflare/{zone.service.ts,types.ts,index.ts}` | Zone API 类型和服务。 |
| `frontend/vitest.config.ts`、`frontend/tests/zone/*.test.tsx` | Zone 页面与域名选择器的最小前端测试运行环境。 |
| `frontend/app/(main)/websites/*` | Zone 列表、`[zoneId]` 动态详情页和局部组件。 |
| `frontend/app/(main)/proxy-routes/*` | Zone 域名选择器替换旧域名/证书编辑器。 |
| `internal/db/migrator/goose/{postgres,sqlite}/202607130001_drop_legacy_route_domain_columns.sql` | 第二阶段删除旧表、列与索引。 |

### Task 1: 第一阶段 Schema、模型与迁移测试

**Files:**
- Create: `internal/db/migrator/goose/postgres/202607120001_create_zone_domain_tables.sql`
- Create: `internal/db/migrator/goose/sqlite/202607120001_create_zone_domain_tables.sql`
- Create: `internal/model/openflare_zone.go`
- Create: `internal/model/openflare_zone_test.go`
- Modify: `internal/model/openflare_proxy_route.go`
- Test: `internal/db/migrator/migrator_test.go`

**Interfaces:**
- Produces: `model.Zone`, `model.ZoneDomain`, `ListZoneDomainsByRouteID(ctx, routeID)`, `ReplaceZoneDomainRouteBindings(ctx, routeID, domainIDs)`.
- Consumes: existing `model.ProxyRoute` and `model.TLSCertificate` IDs; no physical FK.

- [x] **Step 1: 写失败的模型与迁移测试**

```go
func TestReplaceZoneDomainRouteBindingsRejectsForeignDomain(t *testing.T) {
    // Create zones and domains, then assert a domain cannot be bound twice.
}
```

Run: `go test ./internal/model ./internal/db/migrator -run 'Zone|Migrat' -count=1`

Expected: FAIL，因为 Zone 模型和 goose 文件尚不存在。

- [x] **Step 2: 新建双方言 DDL**

```sql
CREATE TABLE IF NOT EXISTS of_zones (
  id BIGSERIAL PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zones_domain ON of_zones (domain);

CREATE TABLE IF NOT EXISTS of_zone_domains (
  id BIGSERIAL PRIMARY KEY,
  zone_id BIGINT NOT NULL,
  proxy_route_id BIGINT,
  domain VARCHAR(255) NOT NULL,
  cert_id BIGINT,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zone_domains_domain ON of_zone_domains (domain);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_zone_id ON of_zone_domains (zone_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_proxy_route_id ON of_zone_domains (proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_cert_id ON of_zone_domains (cert_id);
```

SQLite 使用 `INTEGER PRIMARY KEY AUTOINCREMENT`、`DATETIME`，字段/索引语义完全对齐。此任务不得删除旧列或旧表。

- [x] **Step 3: 实现模型和受事务保护的绑定替换**

```go
type Zone struct { ID uint; Domain string; Remark string; CreatedAt time.Time; UpdatedAt time.Time }
type ZoneDomain struct { ID uint; ZoneID uint; ProxyRouteID *uint; Domain string; CertID *uint; Remark string; CreatedAt time.Time; UpdatedAt time.Time }

func ReplaceZoneDomainRouteBindings(ctx context.Context, routeID uint, domainIDs []uint) error
```

实现先锁定/读取请求域名，拒绝已绑定到其他路由的记录，再把当前路由已绑定但不在 `domainIDs` 的记录置空，最后将请求记录写为 `routeID`；所有动作放在同一 `db.DB(ctx).Transaction` 内。

- [x] **Step 4: 运行模型与迁移测试**

Run: `go test ./internal/model ./internal/db/migrator -run 'Zone|Migrat' -count=1`

Expected: PASS，空 SQLite 库可应用迁移，唯一域名和绑定排他性受保护。

- [x] **Step 5: Commit**

```bash
git add internal/db/migrator/goose internal/model
git commit -m "feat(zone): add normalized zone domain schema"
```

### Task 2: Zone 领域逻辑、历史导入命令与管理 API

**Files:**
- Create: `internal/apps/openflare/zone/{logics.go,routers.go,errs.go,legacy_import.go,logics_test.go}`
- Create: `internal/cmd/migrate_zones.go`
- Create: `internal/router/v1/openflare/register_zone.go`
- Modify: `internal/cmd/root.go`
- Modify: `internal/router/v1/openflare/register_tls.go`
- Test: `internal/apps/openflare/integration/security_test.go`

**Interfaces:**
- Produces: `zone.Create`, `zone.Update`, `zone.GetOverview`, `zone.ImportLegacy(ctx) (ImportReport, error)` and Zone REST handlers.
- Consumes: Task 1 models; legacy `managed_domains` and proxy-route columns only inside `ImportLegacy`.

- [x] **Step 1: 写失败的逻辑与 API 测试**

```go
func TestCreateZoneDomainRejectsWildcard(t *testing.T) { _, err := CreateDomain(ctx, zoneID, DomainInput{Domain: "*.example.com"}); require.EqualError(t, err, errDomainWildcardUnsupported) }
func TestLegacyImportUsesEffectiveTLDPlusOne(t *testing.T) {
    root, err := zoneRoot("api.example.co.uk")
    require.NoError(t, err)
    require.Equal(t, "example.co.uk", root)
}
```

集成测试请求 `POST /api/v1/d/zones/`、`POST /api/v1/d/zones/:id/domains`，并断言错误响应使用 400 信封。

- [x] **Step 2: 实现精确域名和 Zone 归属验证**

```go
func zoneRoot(domain string) (string, error) { return publicsuffix.EffectiveTLDPlusOne(strings.ToLower(strings.TrimSpace(domain))) }
func CreateDomain(ctx context.Context, zoneID uint, input DomainInput) (*model.ZoneDomain, error)
```

拒绝空值、协议、路径和 `*`；要求 `zoneRoot(input.Domain) == zone.Domain`；若 `cert_id` 非空，验证 TLS 证书存在。Zone 根域创建也必须经 `EffectiveTLDPlusOne` 验证且输入等于结果。

- [x] **Step 3: 实现显式导入命令**

```go
var migrateZonesCmd = &cobra.Command{Use: "migrate-zones", RunE: func(_ *cobra.Command, _ []string) error {
    report, err := zone.ImportLegacy(context.Background())
    return report.LogAndReturn(err)
}}
```

导入以事务执行：用 `routeidentity.DecodeDomains(route.Domains, route.Domain)` 读取旧路由；按 `domain_cert_ids` 的同一索引写入 `zone_domains.cert_id`；只在无路由域名时导入旧 `managed_domains`。发现无效根域、通配符记录或全局域名冲突时回滚并输出全部冲突项。重复执行不得生成重复 Zone/ZoneDomain。

- [x] **Step 4: 注册 API 并删除旧 managed-domain 路由**

```go
zoneGroup := apiGroup.Group("/zones")
zoneGroup.Use(apiutil.AdminMiddlewares()...)
zoneGroup.GET("/", zone.ListHandler)
zoneGroup.POST("/", zone.CreateHandler)
zoneGroup.GET("/:id/overview", zone.GetOverviewHandler)
```

把 `managed-domains` 路由块从 `register_tls.go` 移除；每个 Handler 使用 `apiutil.BindJSON` 和 `response.AbortBadRequest/AbortNotFound/AbortConflict`。

- [x] **Step 5: 验证并 Commit**

Run: `go test ./internal/apps/openflare/zone ./internal/apps/openflare/integration -count=1 && make swagger`

Expected: PASS，Swagger 不再含 `/managed-domains` 且包含 `/zones`。

```bash
git add internal/apps/openflare/zone internal/cmd internal/router/v1/openflare docs
git commit -m "feat(zone): add zone management api and legacy importer"
```

### Task 3: 反代路由改用 ZoneDomain 关联

**Files:**
- Modify: `internal/apps/openflare/proxy_route/{logics.go,helpers.go,build_helpers.go,routers.go,errs.go,logics_test.go}`
- Modify: `internal/model/openflare_proxy_route.go`
- Modify: `internal/apps/openflare/tls/logics.go`
- Modify: `internal/apps/openflare/origin/logics.go`

**Interfaces:**
- Consumes: `zone_domain_ids []uint` and Task 1 binding API.
- Produces: `proxy_route.Input{ZoneDomainIDs []uint}`, `proxy_route.View{ZoneDomains []ZoneDomainView}`.

- [x] **Step 1: 写失败的路由逻辑测试**

```go
input := Input{SiteName: "api", ZoneDomainIDs: []uint{domainA.ID, domainB.ID}, EnableHTTPS: true}
view, err := CreateProxyRoute(ctx, input)
require.NoError(t, err)
require.Equal(t, []uint{domainA.ID, domainB.ID}, view.ZoneDomainIDs)
```

同时覆盖：空 `zone_domain_ids`、重复 ID、其他路由已占用域名、HTTPS 域名无证书、证书 SAN 不覆盖。

- [x] **Step 2: 删除路由输入/视图中的旧域名与证书字段**

```go
type ZoneDomainBindingInput struct {
    ZoneDomainIDs []uint `json:"zone_domain_ids"`
}
type ZoneDomainView struct { ID uint `json:"id"`; ZoneID uint `json:"zone_id"`; Domain string `json:"domain"`; CertID *uint `json:"cert_id"` }
```

移除 `Input.Domain`、`Input.Domains`、`Input.CertID`、`Input.CertIDs`、`Input.DomainCertIDs` 及对应 View 字段；删除旧证书派生辅助函数与 `WebsiteService.match` 所需后端逻辑。

- [x] **Step 3: 用关联记录验证并构建路由**

在 `buildProxyRoute` 中读取所有 `ZoneDomainIDs`，对每个 HTTPS 域名调用现有 `validateCertificateCoverage`，再调用 `ReplaceZoneDomainRouteBindings`。更新/删除路由也必须在事务内同步解除关联。来源、证书删除检查和 Origin 路由摘要改从 `zone_domains` 查询域名/证书。

- [x] **Step 4: 运行路由和 TLS 回归测试**

Run: `go test ./internal/apps/openflare/proxy_route ./internal/apps/openflare/tls ./internal/apps/openflare/origin -count=1`

Expected: PASS；任一证书已被 Zone 域名引用时，删除证书被拒绝。

- [x] **Step 5: Commit**

```bash
git add internal/apps/openflare/proxy_route internal/apps/openflare/tls internal/apps/openflare/origin internal/model
git commit -m "refactor(proxy): bind routes through zone domains"
```

### Task 4: 配置快照、渲染与关联消费者

**Files:**
- Modify: `internal/apps/openflare/config_version/{snapshot.go,helpers.go,logics.go,logics_test.go,certificate_snapshot_test.go,pages_snapshot.go}`
- Modify: `pkg/render/openresty/{types.go,render.go,render_route.go,render_test.go}`
- Modify: `internal/apps/openflare/{flared/logics.go,uptimekuma/sync.go}`
- Modify: `internal/apps/openflare/routeidentity/identity.go`

**Interfaces:**
- Produces: snapshot/render `Route{SiteName, Domains, DomainCertIDs}` built transiently from ZoneDomain rows; neither DB model nor API stores those fields.

- [x] **Step 1: 写快照等价性失败测试**

```go
func TestBuildSnapshotReadsZoneDomainCertificates(t *testing.T) {
    // Two explicit ZoneDomains with different certs must render two TLS server blocks.
}
```

加入 Pages、Tunnel、WAF 绑定测试，断言 Route ID 与 `site_name` 未改变。

- [x] **Step 2: 在快照边界联查并生成临时渲染字段**

```go
domains, err := model.ListZoneDomainsByRouteID(ctx, route.ID)
snapshotRoute.Domains = maps.Values(domainNames)
snapshotRoute.DomainCertIDs = certIDsInDomainOrder(domains)
```

`pkg/render/openresty.Route` 可继续保留 `Domains` 与 `DomainCertIDs`，因为它是不可变配置快照的渲染输入；移除其中持久化主域/证书回退逻辑，所有错误消息改用 `SiteName`。

- [x] **Step 3: 移除旧字段回退路径**

删除 `routeidentity.DecodeDomains` 对持久化 `route.Domain` 的依赖；Flared、Uptime Kuma、配置 diff、WAF 文档和 Pages 错误信息都从 snapshot/ZoneDomain 查询的明确域名获取显示文本。

- [x] **Step 4: 运行数据面测试**

Run: `go test ./internal/apps/openflare/config_version ./pkg/render/openresty ./internal/apps/openflare/flared ./internal/apps/openflare/uptimekuma -count=1`

Expected: PASS；迁移后的路由产生的 `server_name`、证书支持文件和 WAF RouteID 绑定与迁移前一致。

- [x] **Step 5: Commit**

```bash
git add internal/apps/openflare/config_version internal/apps/openflare/flared internal/apps/openflare/uptimekuma internal/apps/openflare/routeidentity pkg/render/openresty
git commit -m "refactor(config): render routes from zone domains"
```

### Task 5: Zone 前端服务与 ID 动态页面

**Files:**
- Create: `frontend/lib/services/openflare/zone.service.ts`
- Create: `frontend/vitest.config.ts`
- Create: `frontend/tests/zone/{websites-page.test.tsx,zone-page.test.tsx}`
- Modify: `frontend/lib/services/openflare/{types.ts,index.ts}`
- Modify: `frontend/lib/services/index.ts`
- Modify: `frontend/app/(main)/websites/page.tsx`
- Create: `frontend/app/(main)/websites/[zoneId]/page.tsx`
- Create: `frontend/app/(main)/websites/[zoneId]/components/{zone-overview.tsx,zone-domains-table.tsx,zone-route-summary.tsx,zone-editor-dialog.tsx,zone-domain-dialog.tsx}`
- Delete: `frontend/app/(main)/websites/detail/page.tsx`
- Delete: `frontend/app/(main)/websites/detail/page-client.tsx`
- Delete: legacy Website/managed-domain-only components after imports are removed.

**Interfaces:**
- Produces: `ZoneService.list/getOverview/create/update/delete`, `ZoneDomainService.create/update/delete` and `ZoneOverview` TypeScript types.

- [x] **Step 1: 写服务与页面行为测试**

先安装仅用于本次页面测试的开发依赖：

```bash
cd frontend && pnpm add -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

```ts
expect(ZoneService.getOverview).toHaveBeenCalledWith(42)
expect(screen.getByRole('heading', {name: 'arctel.de'})).toBeVisible()
```

覆盖 `/websites/42` 的加载、404、空域名、搜索列表和从列表点击 ID 链接。

- [x] **Step 2: 实现类型化服务与查询键**

```ts
export interface ZoneDomainItem { id: number; zone_id: number; proxy_route_id: number | null; domain: string; cert_id: number | null; remark: string }
export class ZoneService extends OpenFlareBaseService { protected static override basePath = '/api/v1/d/zones' }
export const zoneQueryKey = ['openflare', 'zones'] as const
```

所有 React Query 回调使用箭头函数，避免静态 service `this` 丢失。

- [x] **Step 3: 用 Next 动态段实现 Zone 详情**

```tsx
export default async function ZonePage({params}: PageProps<'/websites/[zoneId]'>) {
  const {zoneId} = await params
  return <ZonePageClient zoneId={Number(zoneId)} />
}
```

遵循本地 Next 文档：动态 `params` 是 Promise；无效或非正整数 ID 显示既有 `EmptyStateWithBorder`，不把域名写入 URL。主页面只维护页面骨架和 Tabs，具体 Tab 放入同目录组件。

- [x] **Step 4: 实现列表和详情交互**

列表仅渲染 Zone 根域及计数；详情使用概览、域名、路由、证书、设置 Tabs。域名弹窗拒绝 `*.`，但证书选择器不限制其 SAN。删除 Zone/域名使用确认对话框和服务端错误文案。

- [x] **Step 5: 验证并 Commit**

Run: `cd frontend && pnpm exec vitest run && pnpm lint`

Expected: PASS。

```bash
git add frontend/lib/services frontend/app/'(main)'/websites
git commit -m "feat(web): add zone-based website management"
```

### Task 6: 反代路由前端切换到 Zone 域名选择器

**Files:**
- Create: `frontend/app/(main)/proxy-routes/components/zone-domain-selector.tsx`
- Create: `frontend/tests/zone/zone-domain-selector.test.tsx`
- Modify: `frontend/app/(main)/proxy-routes/{components/proxy-route-create-sheet.tsx,components/helpers.ts,page-client.tsx}`
- Modify: `frontend/app/(main)/proxy-routes/detail/{helpers.ts,page-client.tsx,components/domain-section.tsx}`
- Delete: `frontend/app/(main)/proxy-routes/detail/components/domain-list-input.tsx`
- Modify: `frontend/lib/services/openflare/types.ts`

**Interfaces:**
- Consumes: `ZoneDomainItem[]` and route `zone_domain_ids: number[]`.
- Produces: selector values with explicit domain/Zone/证书信息；不发送任何旧域名或证书字段。

- [x] **Step 1: 写失败的选择器测试**

```tsx
render(<ZoneDomainSelector value={[7]} onChange={onChange} domains={[apiDomain]} />)
expect(screen.getByText('api.arctel.de')).toBeVisible()
expect(onChange).toHaveBeenCalledWith([7])
```

覆盖搜索、跨 Zone 多选、已被其他路由占用的禁用项和 HTTPS 缺少证书的表单错误。

- [x] **Step 2: 移除旧前端负载与自动匹配**

从 `ProxyRouteItem`/`ProxyRouteMutationPayload` 删除 `domain`、`domains`、`primary_domain`、`cert_id`、`cert_ids`、`domain_cert_ids`；删除 `WebsiteService.match` 及 `DomainListInput` 自动填证书交互。

- [x] **Step 3: 实现 Zone 域名选择和保存负载**

```ts
mutationFn: (payload) => ProxyRouteService.update(route.id, {
  ...payload,
  zone_domain_ids: selectedDomainIDs,
})
```

展示每个选择项的 FQDN、所属 Zone 与证书；路由详情的“域名”区只编辑关联关系，证书链接跳转 Zone 详情而非路由内编辑。

- [x] **Step 4: 运行前端类型和交互测试**

Run: `cd frontend && pnpm exec tsc --noEmit`

Expected: PASS；不存在旧持久化域名/证书字段的 TypeScript 引用。

- [x] **Step 5: Commit**

```bash
git add frontend/app/'(main)'/proxy-routes frontend/lib/services/openflare/types.ts
git commit -m "refactor(web): select route domains from zones"
```

### Task 7: 第一发布阶段验证、文档与发布前数据检查

**Files:**
- Modify: `docs/design/zone-design.md`
- Modify: `docs/changelog/index.md`
- Modify: generated `docs/{docs.go,swagger.json,swagger.yaml}`
- Create: `docs/guide/zone-domain-migration.md`

- [x] **Step 1: 为导入命令写可操作迁移指南**

文档写明备份、执行 `wavelet migrate-zones`、读取导入报告、发布预览、比较 `server_name`/证书支持文件、发布激活和回滚步骤；不允许在报告有冲突时继续。

- [x] **Step 2: 生成 Swagger 和更新未发布变更**

Run: `make swagger`

在 `[Unreleased]` 记录 Zone 管理、反代路由域名正规化和移除 managed-domain API。

- [x] **Step 3: 运行全量质量门禁**

Run: `go test ./... && make code-check`

Expected: PASS。

- [x] **Step 4: 做快照等价性验收**

在升级前导出活动版本，在导入后生成预览；逐个比较所有路由的明确 `server_name` 集合、证书路径、WAF RouteID 绑定与 Pages 部署引用。只允许旧快照的域名/证书冗余 JSON 消失，不允许数据面语义变化。

- [x] **Step 5: Commit**

```bash
git add docs
git commit -m "docs(zone): add migration and release verification guide"
```

### Task 8: 第二发布阶段——删除旧表和冗余列

**Precondition:** 已在生产环境完成 Task 7 的导入、预览对比和至少一次发布/回滚验证；`migrate-zones` 报告无冲突。

**Files:**
- Create: `internal/db/migrator/goose/postgres/202607130001_drop_legacy_route_domain_columns.sql`
- Create: `internal/db/migrator/goose/sqlite/202607130001_drop_legacy_route_domain_columns.sql`
- Delete: `internal/model/openflare_managed_domain.go`
- Delete: `internal/apps/openflare/tls/managed_domain.go`
- Delete: `internal/apps/openflare/tls/helpers.go` 中仅用于旧路由证书数组的函数
- Modify: legacy迁移相关测试、模型测试与 `docs/design/zone-design.md`

- [x] **Step 1: 写空库与升级库清理失败测试**

```go
func TestLegacyRouteColumnsAreAbsentAfterCleanup(t *testing.T) {
    require.False(t, db.DB(ctx).Migrator().HasColumn(&model.ProxyRoute{}, "domain"))
}
```

- [x] **Step 2: 编写双方言清理 DDL**

PostgreSQL 删除旧唯一索引和 `domain`、`domains`、`cert_id`、`cert_ids`、`domain_cert_ids`，再删除 `of_managed_domains`；SQLite 使用重建 `of_proxy_routes` 表的迁移方式保留所有非旧字段与索引。Down 仅在开发数据库恢复旧结构，不回填历史数据。

- [x] **Step 3: 删除旧读取代码与测试 fixture**

删除所有 `route.Domain`、`route.Domains`、`route.CertID`、`route.CertIDs`、`route.DomainCertIDs` 的持久化引用；让编译器、Uptime Kuma、Flared、来源摘要及 API 只使用 ZoneDomain 查询结果。

- [x] **Step 4: 验证升级和完整回归**

Run: `go test ./internal/db/migrator ./internal/model ./internal/apps/openflare/... ./pkg/render/openresty -count=1 && make code-check`

Expected: PASS；全仓搜索不再发现旧 `ManagedDomain` 业务代码、`ProxyRoute` 持久化字段或管理端 API；渲染快照中的临时 `DomainCertIDs` 类型允许保留。

- [x] **Step 5: Commit**

```bash
git add internal/db/migrator internal/model internal/apps frontend docs
git commit -m "refactor(zone): remove legacy route domain storage"
```

## Plan Self-Review

* Spec coverage: Tasks 1–4 交付正规化数据模型、API、迁移与数据面；Tasks 5–6 交付 ID 路由和 Zone 交互；Tasks 7–8 覆盖质量门禁与旧表清理。
* Placeholder scan: 无待定标记或未定义的实现步骤；所有删除动作在明确的生产验证前置条件后执行。
* Type consistency: 路由写入统一使用 `zone_domain_ids`，持久化关系统一为 `ZoneDomain.ProxyRouteID`，渲染边界仅使用临时 `Domains`/`DomainCertIDs`。
