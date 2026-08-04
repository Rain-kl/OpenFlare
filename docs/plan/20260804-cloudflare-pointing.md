# Cloudflare DNS 指向实现计划

> 状态：代码实施完成（2026-08-04；范围内自动化验证完成；全量前端仅保留任务开始前已存在的 Zone 文案断言失败）

> **执行方式**：使用 `superpowers:executing-plans` 在当前会话按任务逐项实施；每项遵循测试先行（RED → GREEN → REFACTOR）。

## 1. 目标与背景 (Goal & Context)

* **需求背景**：落实提交 `21fb303e` 中的 Cloudflare DNS 指向设计，让管理员以 ZoneDomain 为粒度，将明确 FQDN 的单条 A 记录幂等指向 OpenFlare 边缘节点 IPv4，避免在 Cloudflare 控制台重复手工操作。
* **开发范围 (Scope)**：
  * 全局一份 Cloudflare 连接，支持从现有 Cloudflare DNS 账号导入或独立录入 API Token。
  * 指向分组、成员、主/备/生效节点、成员橙云、同步状态与错误信息。
  * Cloudflare Zone/DNS Record HTTP 客户端与单成员幂等 reconcile。
  * 手动同步、成员/分组变更同步、节点 IP 变化 best-effort 入队。
  * 管理 API、Swagger、前端总览/设置/分组列表/分组详情和侧边栏入口。
* **Out of Scope**：自动故障切换/回切、AAAA、多 A 负载、CNAME、定时全量对账、非 Cloudflare DNS 厂商、多 Cloudflare 账号并行。

## 2. 设计与决策 (Design & Decisions)

### 核心对象/数据模型

* `of_cf_connections`：全局连接；`source` 为 `dns_account` 或 `standalone`，独立 Token 使用现有 `enc:v1:` 密文格式，响应永不暴露凭据。
* `of_cf_pointing_groups`：分组名、主节点、可选备用节点、生效节点、默认橙云和启用状态；一期 `active_node_id = primary_node_id`。
* `of_cf_pointing_members`：分组、全局唯一 `zone_domain_id`、成员橙云、Cloudflare Zone/Record ID 缓存、期望 IP 与同步状态。
* PostgreSQL/SQLite 使用同版本 Goose DDL，不建立物理外键，关系字段显式索引，数据库默认值与 Go 零值一致。

### API 与鉴权设计

* 前缀 `/api/v1/d/cloudflare`，统一使用 `apiutil.AdminMiddlewares()`。
* 连接：`GET/PUT /connection`、`POST /connection/verify`、`POST /connection/clear`。
* 总览：`GET /overview`。
* 分组：`GET/POST /groups`、`GET /groups/:id`、`POST /groups/:id/update|delete|sync`。
* 成员：`GET/POST /groups/:id/members`、`POST /groups/:id/members/:memberId/update|remove|sync`。
* 可用域名：`GET /domains/available`。
* 成功统一 HTTP 200 + `response.OK`；失败通过 `response.Abort*` 交由全局中间件写出。

### 数据流与架构图

```mermaid
flowchart LR
  UI[Cloudflare 管理页面] --> API[/api/v1/d/cloudflare]
  API --> Logic[cloudflare 业务逻辑]
  Logic --> Repo[repository]
  Repo --> DB[(PG / SQLite)]
  Logic --> Queue[Asynq]
  Queue --> Worker[Cloudflare 同步 Handler]
  Worker --> Reconcile[成员 Reconcile]
  Reconcile --> CF[Cloudflare Zone / DNS API]
  Reconcile --> Repo
  Node[节点手动更新或心跳] --> Queue
```

### 设计决策权衡

* 使用标准库 `net/http` 自建最小 Cloudflare 客户端，避免引入覆盖面过大的 SDK；接口只暴露 verify、Zone 查找和 A 记录 CRUD，便于 mock。
* 单成员同步采用进程内 keyed mutex 防止同一 Worker 进程并发双写；数据库状态在调用远端前标记 `syncing`，完成后写回 `ok/error`。
* 整组、按节点和常规变更统一通过 Asynq 投递单成员任务；请求路径只做校验和状态变更，避免管理 API 被远端网络延迟阻塞。连接测试是唯一同步调用 Cloudflare 的管理操作。
* 删除成员/分组默认先删除已缓存或唯一同名 A 记录，再删除本地记录；远端删除失败时保留本地成员并返回可读错误，避免失去重试依据。
* 将 TLS 包内的敏感字段加解密提取为 `internal/apps/openflare/credential`，保持既有密文兼容并让 Cloudflare 复用，避免业务包重复实现凭据存储。

## 3. 具体修改文件清单 (Proposed Changes)

### Task 1：凭据共享与数据库模型

**测试先行**：验证旧明文、`enc:v1:` 密文、无 SessionSecret 和缺少密钥时的兼容行为；验证迁移能创建三张表和唯一索引。

* **[NEW]** `internal/apps/openflare/credential/sensitive.go`、`sensitive_test.go`：提供 `Seal` / `Open`。
* **[MODIFY]** `internal/apps/openflare/tls/sensitive.go` 及调用点：委派到共享凭据包，保留 TLS 对外行为。
* **[NEW]** `internal/model/openflare_cloudflare.go`：连接、分组、成员实体和同步状态常量。
* **[NEW]** `internal/infra/persistence/migrator/goose/{postgres,sqlite}/202608040001_create_cloudflare_pointing.sql`。
* **[MODIFY]** `internal/infra/persistence/migrator/migrator_test.go`：检查表、索引和唯一约束。

### Task 2：Repository 与 Cloudflare HTTP 客户端

**测试先行**：覆盖连接 upsert/clear、分组/成员 CRUD、可用域名、按 active node 查询成员；使用 `httptest.Server` 覆盖 Token verify、Zone 查找、A 记录 list/create/update/delete、API 错误和 429 `Retry-After`。

* **[NEW]** `internal/repository/openflare_cloudflare.go`、`openflare_cloudflare_test.go`：唯一持久化入口和必要事务。
* **[NEW]** `internal/apps/openflare/cloudflare/client.go`、`client_test.go`：最小 Cloudflare API 接口与 HTTP 实现。
* **[NEW]** `internal/apps/openflare/cloudflare/types.go`、`errs.go`：输入/输出 DTO、内部状态和用户可见错误常量。

### Task 3：Reconcile、业务逻辑与异步任务

**测试先行**：覆盖 Token 来源解析、0/1/多条同名 A、缓存 Record ID 失效回退、非法 IPv4、成员默认橙云初始化、成员更新、移出删除远端、分组变更入队、按节点 IP 变更入队和同成员串行执行。

* **[NEW]** `internal/apps/openflare/cloudflare/reconcile.go`、`reconcile_test.go`：单成员期望状态计算与幂等同步。
* **[NEW]** `internal/apps/openflare/cloudflare/logics.go`、`logics_test.go`：连接、总览、分组、成员业务编排。
* **[NEW]** `internal/apps/openflare/cloudflare/tasks.go`、`tasks_test.go`：`cloudflare:sync_member`、`sync_group`、`sync_by_node` Handler、Meta、payload 校验与投递函数。
* **[MODIFY]** `internal/infra/task/handlers/register.go`：集中注册 Cloudflare 任务。
* **[MODIFY]** `internal/apps/openflare/node/logics.go`、`internal/apps/openflare/agent/logics.go`：节点 IP 真正变化后 best-effort 投递，不阻断原流程；失败记录日志。

### Task 4：管理 API 与 Swagger

**测试先行**：使用 Gin 测试覆盖管理员路由、参数绑定、404/409/未就绪映射、Token 响应脱敏和主要成功响应。

* **[NEW]** `internal/apps/openflare/cloudflare/routers.go`、`routers_test.go`：Handlers 与 Swagger 注释。
* **[NEW]** `internal/router/v1/openflare/register_cloudflare.go`。
* **[MODIFY]** `internal/router/v1/openflare/v1.go`：注册 Cloudflare 路由委派。
* **[GENERATED]** `docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml`：运行 `make swagger` 生成。

### Task 5：前端服务、导航与页面

**测试先行**：覆盖 service 路径/载荷、未就绪引导、连接配置不回显 Token、分组创建、成员添加/橙云更新、同步与删除确认。

* **[NEW]** `frontend/lib/services/openflare/cloudflare.service.ts`。
* **[MODIFY]** `frontend/lib/services/openflare/types.ts`、`index.ts`：类型、导出和 `openflareServices.cloudflare`。
* **[MODIFY]** `frontend/lib/navigation/openflare-nav.ts`：网站管理组增加 Cloudflare 入口与子路由高亮。
* **[NEW]** `frontend/app/(main)/cloudflare/page.tsx`：总览和就绪门禁。
* **[NEW]** `frontend/app/(main)/cloudflare/settings/page.tsx`：DNS 账号导入/独立 Token 配置与连接测试。
* **[NEW]** `frontend/app/(main)/cloudflare/groups/page.tsx`：分组列表、创建、同步、删除确认。
* **[NEW]** `frontend/app/(main)/cloudflare/groups/[id]/page.tsx` 及邻近 `components/`：分组配置、成员列表、添加/更新/同步/移除。
* **[NEW]** `frontend/tests/cloudflare/*.test.ts(x)`：服务和关键交互测试。

### Task 6：设计边界、变更日志与收尾

* **[MODIFY]** `docs/design/architecture.md`、`docs/design/index.md`：补充 Cloudflare 可选控制面能力与阅读入口。
* **[MODIFY]** `docs/changelog/index.md`：在 `[Unreleased]` 添加中文用户可见条目。
* **[MODIFY]** `docs/plan/index.md`：登记本计划；完成时保留计划并标记状态。

## 4. 验证计划 (Verification Plan)

### 自动化单元测试

* `go test ./internal/apps/openflare/credential ./internal/apps/openflare/cloudflare ./internal/repository ./internal/infra/persistence/migrator ./internal/apps/openflare/node ./internal/apps/openflare/agent`
* `pnpm --dir frontend test -- --run frontend/tests/cloudflare`
* `go test ./...`

### 生成与质量门禁

* `make license`
* `make swagger`
* `make format`
* `make code-check`

### 手动验收路径

1. 在 `/cloudflare/settings` 选择现有 Cloudflare DNS 账号或录入独立 Token，测试连接成功。
2. 在 `/cloudflare/groups` 新建分组，选择具有合法 IPv4 的 edge 节点。
3. 在详情页加入 ZoneDomain，观察状态从 `pending/syncing` 变为 `ok`，Cloudflare 上出现单条 A 记录。
4. 修改成员橙云并同步，确认远端 `proxied` 与期望一致。
5. 构造同名多 A，确认同步失败并提示先在 Cloudflare 清理。
6. 移出成员，确认默认删除本模块管理的远端 A；修改节点 IP 后确认相关成员重新入队。

## 5. 实施结果与验证记录

* 已完成共享凭据加密、双数据库迁移、repository、Cloudflare HTTP 客户端、成员 reconcile、三类异步任务、管理 API、节点 IP 变化联动、前端服务与四级管理页面。
* 删除远端记录时，缓存 Record ID 失效会回退到唯一同名 A；停用分组内修改成员只标记 `pending`，不投递必然失败的同步任务。
* 已运行 `make license`、`make swagger`、`make format`；Swagger 已生成 Cloudflare 管理接口。
* `go test ./...` 通过。
* `make code-check` 通过，包含架构守卫、golangci-lint、TypeScript 与 ESLint。
* `pnpm exec vitest run tests/cloudflare` 通过（2 个测试文件、2 个测试）。
* 前端全量 Vitest 为 20/21 个测试文件、106/107 个测试通过；唯一失败为既存 `tests/zone/zone-page.test.tsx` 仍断言页面展示“唯一访问者”，与本功能无关且在本任务基线中已存在。
