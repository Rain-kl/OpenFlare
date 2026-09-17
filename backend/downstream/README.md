# 下游插件开发指南 (Downstream Custom Plugins)

本目录为 Wavelet 下游业务定制插件（Deployment-specific plugins）的专属开发目录。

所有插件开发均以标准模板 [`custom_example`](./plugins/custom_example) 为基准进行构建。

---

## 目录结构

```text
downstream/
├── README.md
└── plugins/
    └── custom_example/       # 标准插件开发基准模板（可直接复制并重命名开发新插件）
        ├── plugin.go         # 插件入口：实现 core.Plugin (Name & Apply)
        ├── consts/           # 常量与错误码定义
        │   └── consts.go
        ├── controller/       # 控制器层 (HTTP API 接口、参数绑定与信封响应)
        │   └── hello/
        │       └── hello.go
        ├── service/          # 业务逻辑层 (用例编排、事务控制与领域事件)
        ├── dao/              # 数据访问层 (GORM CRUD、SQL 防注入与转义)
        ├── model/            # 数据模型与实体定义
        │   ├── do/           # Domain Object 领域对象与 DTO
        │   └── entity/       # 数据表映射实体 (TableName() 带专属前缀)
        └── migrations/       # Goose SQL 双方言独立数据库迁移
            ├── postgres/     # PostgreSQL 迁移脚本
            └── sqlite/       # SQLite 迁移脚本
```

---

## 插件开发规范与分层职责

每个下游插件均需遵循标准分层架构（`controller -> service -> dao -> model`）：

1. **`plugin.go` (插件装配入口)**：
   - 实现 `core.Plugin` 接口（`Name() string` 与 `Apply(ctx *core.Context) error`）。
   - 负责在 `Apply` 中注册路由组（`ctx.Router()`）、异步任务（`ctx.Task()`）、定时调度（`ctx.Schedule()`）与数据库迁移（`ctx.Migrations()`）。
   - 依赖注入统一使用 `core.Inject` 或 `ctx.Using` 获取平台服务（如 `contracts.AuthService`、`contracts.DBService`、`contracts.CacheService`）。

2. **`controller/` (控制器层 / Handler)**：
   - 负责 HTTP API 请求参数绑定（`c.ShouldBindJSON` / `c.ShouldBindQuery`）、用户会话获取（`oauth.GetCurrentUser`）。
   - 调用 Service 层处理业务，严禁直接包含复杂业务逻辑或直接执行 SQL 操作。
   - 统一使用 `response.OK` 或 `response.Abort*` 返回标准信封响应。

3. **`service/` (业务逻辑层)**：
   - 纯 Go 业务用例，方法入参首位统一为 `context.Context`。
   - 严禁依赖 `*gin.Context` 或 HTTP 相关对象，确保逻辑具备可移植性与可测试性。
   - 涉及数据修改可通过 `ctx.Events().Emit()` 发射强类型领域事件。

4. **`dao/` (数据访问层 / Repository)**：
   - 负责底层数据库交互，通过 `contracts.DBService` 获取受保护的 GORM 数据库句柄。
   - 模糊查询必须调用 `pkg/util.EscapeLike` 并显式声明 `ESCAPE '\\'` 防注入。
   - 遵循**表单一所有者原则**：严禁跨过其他所有者插件直读或修改其他插件的数据表。

5. **`model/` (模型与 DTO)**：
   - `model/entity/`：数据库表映射结构体，`TableName()` 必须带有插件专属前缀（如 `w_custom_*`）。
   - `model/do/`：业务领域对象、入参校验 Request DTO 与响应 Response DTO。

6. **`consts/` (常量与错误定义)**：
   - 定义插件内部常量、配置键名及驼峰式（camelCase）错误标识字符串。

7. **`migrations/` (双方言 SQL 迁移)**：
   - 包含 `postgres/` 与 `sqlite/` 双方言 Goose SQL 迁移文件，使用 `//go:embed` 打包并在 `Apply()` 中注册。

---

## 快速上手

### 1. 基于模板复制创建新插件

```bash
cp -r backend/downstream/plugins/custom_example backend/downstream/plugins/my_plugin
```

### 2. 实现插件入口 (`plugin.go`)

```go
package my_plugin

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return "my_plugin"
}

func (p *Plugin) Apply(ctx *core.Context) error {
	// 通过容器解析认证服务
	var authSvc contracts.AuthService
	if err := core.Using[contracts.AuthService](ctx, func(svc contracts.AuthService) { authSvc = svc }); err != nil {
		return err
	}

	// 注册带鉴权中间件的路由组
	g := ctx.Router().Group("/api/v1/my-plugin", authSvc.RequireAuthMiddleware().(gin.HandlerFunc))
	g.GET("/hello", func(c *gin.Context) {
		user, err := authSvc.GetCurrentUser(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Hello " + user.Username})
	})

	return nil
}
```

### 3. 在应用装配入口注册插件 (`cmd/app.go`)

在 `cmd/app.go` 中的 `newWaveletApp` 函数内注册你的新插件：

```go
app.Use(
    database.New(),
    cache.New(),
    logger.New(),
    storage.New(),
    // ... 官方平台插件 ...
    my_plugin.New(), // 注册下游定制插件
    driver_http.New(),
    driver_asynq_worker.New(),
    driver_asynq_cron.New(),
)
```

---

## 严格红线与规范 (Guardrails)

- **严禁跨插件直接 import**：下游插件可依赖 `core/`、`core/contracts/`、`pkg/`、`plugins/infra/`，**严禁直接 import `plugins/domain/*` 内部私有实现**，一律通过 `contracts` 接口或事件总线调用。
- **禁止 GORM AutoMigrate**：数据表结构定义必须通过 `migrations/` 下嵌入的 Goose SQL 管理。
- **Goroutine 并发安全**：严禁裸 `go func()`，后台并发任务统一使用 `backend/pkg/util.Go`。
