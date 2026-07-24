---
name: "new-setting"
description: "Wavelet 项目专用：当新增或修改启动时设置、数据库系统设置、业务设置、公共可见配置、/admin/system 参数配置、/admin/settings 图形化设置界面，或前端公共配置消费逻辑时必须使用。本技能指导设置类型判定、SystemConfig 字段与 visibility、goose SQL 初始化/升级、热更新读取、公共配置暴露、shadcn 图形组件和验证流程。"
---

# 新增设置项

本技能覆盖 Wavelet 的设置体系。开始前先读仓库根目录 `AGENTS.md`，遵守项目级规则：HTTP 路由只在 `internal/router/router.go` 注册、API 变更后运行 `make swagger`、提交前运行 `make code-check`、不要删除 `frontend/node_modules`、`internal/util/` 不引入框架依赖。

如果需要在 `/admin/settings` 增加或调整图形化设置组件，同时阅读 [shadcn](../shadcn/SKILL.md)。如果只是新增 Go 读取逻辑、测试或错误处理，再按需阅读对应 `go-*` skill。

## 先判定设置类型

Wavelet 当前有两套设置入口：

- 启动时设置：来自 `config.yaml` 或环境变量，适合进程启动前必须确定、通常不热更新的基础配置。
- 系统设置：保存于数据库 `system_configs`，经 `model.SystemConfig` 实体（key 常量在 model）与 `repository` 读取层（含 Redis hash 缓存）访问，支持运行时热更新。管理入口是 `/admin/system` 和 `/admin/settings`。

系统设置分三种使用语义：

- 业务设置：`type=business`，由管理员配置，影响业务规则，例如用户额度、业务限制。
- 系统设置：`type=system`，由管理员配置，影响平台能力、基础开关、外部服务参数。
- 公共可见配置：附加在业务设置或系统设置之上，由 `visibility=1` 控制是否通过公开接口返回给前端使用。它不是第三种数据库 `type`，不要把 `type` 写成 `public`。

业务设置和系统设置互斥：一个配置项只能选择 `business` 或 `system`。是否公开给前端由 `visibility` 决定：`0` 表示隐藏，`1` 表示 `/api/v1/config/public` 可见。

特殊设置组件不一定需要新增 `SystemConfig` 参数项。例如认证源设置、模板管理这类有独立模型和 API 的功能，应沿用对应领域模型，不要为了出现在 `/admin/settings` 强行创建参数配置。

## 先定位真实链路

修改前快速查看这些文件，确认当前实现没有漂移：

- `internal/model/system_configs.go`: 配置 key 常量（`ConfigKey*`）、`SystemConfig` 实体与字段语义；**不含**持久化读取 API。
- `internal/repository/system_config.go`: 配置读取与缓存（`GetSystemConfigByKey`、`GetBoolByKey`、`GetIntByKey`、`GetDecimalByKey`、`ListVisibleSystemConfigs` 等）。
- `internal/infra/persistence/migrator/goose/postgres/*.sql` 和 `internal/infra/persistence/migrator/goose/sqlite/*.sql`: `system_configs` 表结构、初始化 seed、后续升级迁移。
- `internal/infra/persistence/migrator/migrator.go`: goose 迁移入口和 PostgreSQL/SQLite 方言选择。
- `internal/testhelper/test_helper.go`: Go 测试用默认系统配置 seed。
- `internal/apps/admin/system_config/routers.go`: `/api/v1/admin/system-configs` 参数表 API。
- `internal/apps/config/routers.go`: `/api/v1/config/public` 公共配置响应。
- `frontend/components/common/admin/system.tsx`: `/admin/system` 参数表管理界面，展示所有参数配置项。
- `frontend/components/common/settings/system-settings.tsx`: `/admin/settings` 图形化设置页入口。
- `frontend/components/common/settings/*-tab.tsx`: `/admin/settings` 各图形化设置分组。
- `frontend/lib/services/admin/*`: Admin 系统配置 service 类型和 API 封装。
- `frontend/lib/services/config/*`、`frontend/hooks/use-public-config`、`frontend/components/layout/*`: 前端公共配置消费链路。

## 新增数据库系统设置

按影响面选择步骤，不要只改 UI 或只改默认值。

1. 定义配置 key。
   - 在 `internal/model/system_configs.go` 添加 `ConfigKey...` 常量。
   - key 使用 lowercase snake case，例如 `search_engine_indexing_enabled`。
   - 值仍存为字符串；布尔值用 `"true"` / `"false"`，数值用十进制字符串，复杂结构用 JSON 字符串。

2. 初始化默认配置。
   - 如果修改初始 schema，必须同步 `internal/infra/persistence/migrator/goose/postgres/` 和 `internal/infra/persistence/migrator/goose/sqlite/` 中的 goose SQL。
   - 既有库新增配置时，新增一组时间戳递增的双 SQL 迁移文件，分别放在 PostgreSQL 和 SQLite 目录；不要回到 GORM AutoMigrate 或 Go 代码 seed。
   - 新库初始化也需要包含同一个默认 key：当前初始 seed 在 `202606090001_initial_schema.sql` 的 `INSERT INTO system_configs (...) VALUES ... ON CONFLICT (key) DO NOTHING`。
   - 设置正确的 `Type`：只能是 `"system"` 或 `"business"`。
   - 设置正确的 `Visibility`：公共可见填 `1`，内部配置填 `0`。
   - 默认值要和 Go 读取侧的零值或兜底值一致，避免首次启动和数据库缺失时行为不同。
   - 如果相关 Go 包测试依赖默认配置，同步 `internal/testhelper/test_helper.go` 的 `seedDefaultConfigs` 和公共 key 列表。

3. 读取配置。
   - 后端业务代码通过 `internal/repository` 读取：`repository.GetBoolByKey`、`repository.GetIntByKey`、`repository.GetDecimalByKey` 或 `repository.GetSystemConfigByKey`；key 常量仍用 `model.ConfigKey*`。
   - 禁止新增或调用 `model.Get*ByKey` / `model.ListVisibleSystemConfigs` 等数据访问 API（model 无 CRUD）。
   - 运行时可热更新的规则不要放进 `config.Config`；启动时设置才走 `internal/infra/config/model.go` 和 `config.example.yaml`。
   - 不要在 handler 或业务代码里直接读 `os.Getenv()`。

4. 如果前端需要未登录或全局消费，暴露为公共可见配置。
   - 把该配置的 `visibility` 设为 `1`，`GetPublicConfig` 会通过 `repository.ListVisibleSystemConfigs` 返回所有可见 key/value。
   - `/api/v1/config/public` 的 `data` 是动态对象：后端返回 `map[string]string`，前端类型是 `Record<string, string | undefined>`。
   - 前端读取时按配置 key 访问，必要时在消费侧把字符串转换为 boolean/number/JSON。
   - 检查使用方的 query key，更新后需要 invalidate `["public-config"]`。
   - 只有公共配置 API 形状或注释变化时才需要更新 Swagger；单纯新增 `visibility=1` 的 key 通常不需要改 `PublicConfigResponse` 类型。

5. 如果管理员需要图形化配置，更新 `/admin/settings`。
   - 先阅读 shadcn skill。
   - 根据设置语义选择现有 tab：安全类进 `security-tab.tsx`，运营类进 `operation-tab.tsx`，系统基础参数进 `system-tab.tsx`，其它菜单或杂项进 `other-tab.tsx`。
   - `SystemSettingsMain` 当前通过 `AdminService.listSystemConfigs("system")` 只加载 `type=system` 的配置；`type=business` 的配置若也需要图形化入口，先确认是否要调整查询范围或放到其它 Admin 页面。
   - 新的图形组件优先放在 `frontend/components/common/settings/`，使用现有 `AdminService.updateSystemConfig`。
   - 更新成功后 invalidate `["admin", "system-configs"]`；公共可见配置还要 invalidate `["public-config"]`。
   - 使用 Sonner toast 反馈成功或失败。
   - 不使用 `any`，不要硬编码页面级 `max-w-*`，页面根容器保持 `w-full`。

6. `/admin/system` 参数表通常不需要新代码。
   - 只要 `SystemConfig` 默认数据存在，参数表会展示配置项。
   - `/admin/system` 偏向所有参数配置项的键值管理，不替代 `/admin/settings` 的友好图形界面。

## 新增启动时设置

只有在配置必须随进程启动确定、不能或不应热更新时，才走启动时设置。

1. 在 `internal/infra/config/model.go` 添加配置字段。
2. 在 `config.example.yaml` 添加示例值和说明。
3. 确认 Viper 现有加载逻辑能绑定该字段；需要环境变量时沿用当前命名和绑定方式。
4. 运行时代码从 `config.Config.<Section>.<Field>` 读取。
5. 不要把启动时设置同步塞进 `SystemConfig`，除非产品明确需要运行时覆盖。

## 常见模式

### 布尔公共设置

- model key：`ConfigKeyFeatureEnabled = "feature_enabled"`（定义在 `internal/model`）
- goose SQL 默认值：`value='false'`，`type` 按语义选 `"system"` 或 `"business"`，`visibility=1`。
- 后端读取：`repository.GetBoolByKey(ctx, model.ConfigKeyFeatureEnabled)`。
- 公共响应：`/api/v1/config/public` 的 `data.feature_enabled` 为字符串 `"true"` 或 `"false"`。
- 前端图形控件：`Switch`，保存时写 `"true"` / `"false"`。

### 数值业务设置

- model key：`ConfigKeyMaxSomething = "max_something"`。
- goose SQL 默认值：例如 `"5"`，`type` 通常为 `"business"`，只有前端公共消费时才设 `visibility=1`。
- 后端读取：`repository.GetIntByKey` 或 `repository.GetDecimalByKey`。
- 前端图形控件：`Input type="number"` 或合适的 shadcn 数值控件；保存前做最小必要校验，错误用 toast。

### JSON 设置

- 默认值使用合法 JSON，例如 `"{}"` 或 `"[]"`。
- 在 repository 或业务 logics 中提供解析函数，像 `repository.GetMenuDisplayConfig` 一样把 JSON 解析错误包装成清晰错误；不要在 model 中做 IO。
- 前端不要直接拼接 JSON 字符串；用 `JSON.stringify` 写入，用类型化对象在组件中操作。

## 验证

根据改动范围运行最小有效验证，最后提交前必须运行项目门禁。

- 新增或修改系统配置默认值、visibility 或公共配置读取：至少运行相关 Go 包测试，例如：

```bash
go test ./internal/repository ./internal/apps/config ./internal/apps/admin/system_config
```

- 新增 goose 迁移后，至少用当前数据库方言跑一次迁移；如果 SQL 同时改了 PostgreSQL 和 SQLite，尽量覆盖两种方言。涉及 schema/seed 的任务还应遵循 database-migration skill。

- 公共配置 API 注释或 handler 签名改动后：

```bash
make swagger
```

- 前端图形设置改动后：

```bash
cd frontend && pnpm typecheck && pnpm lint
```

- 提交前：

```bash
make code-check
```

如涉及前端页面体验，启动本地服务并用浏览器验证 `/admin/settings` 和 `/admin/system`：配置能显示、保存、toast 反馈正常、刷新后值保持、公共配置消费方能即时或刷新后生效。

## 相关 Skills

- shadcn：新增或调整 `/admin/settings` 图形化设置组件时使用。
- database-migration：新增或修改 `system_configs` schema、默认 seed 或 goose SQL 迁移时使用。
- go-error-handling：配置解析、缺失配置、非法值错误需要跨包返回时使用。
- go-testing：为配置读取、公共配置 API 或 Admin 配置 API 添加测试时使用。
- go-context：配置读取在请求链路或后台链路中传递取消和超时时使用。
